package transform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	pc "github.com/egorka-gh/pixlpark/photocycle"
	pp "github.com/egorka-gh/pixlpark/pixlpark/service"
)

//can't detect exactly if item is photo product
//if something goes wrong just says errCantTransform
//TODO add alias to mark photo products?
func (fc *baseFactory) transformPhoto(ctx context.Context, item pp.OrderItem, order *pc.Order) error {
	p, ok := item.Sku()["paper"]
	if !ok || p == "" {
		return ErrCantTransform{errors.New("Не указан алиас бумаги (paper)")}
	}
	paper, err := strconv.Atoi(p)
	if err != nil || paper == 0 {
		return ErrTransform{fmt.Errorf("Не верное значение алиаса бумаги (paper) %s", p)}
	}

	w, ok := item.Sku()["width"]
	if !ok || w == "" {
		return ErrCantTransform{errors.New("Не указан алиас ширины (width)")}
	}
	width, err := strconv.Atoi(w)
	if err != nil || width == 0 {
		return ErrTransform{fmt.Errorf("Не верное значение алиаса ширины (width) %s", w)}
	}

	h, ok := item.Sku()["height"]
	if !ok || h == "" {
		return ErrCantTransform{errors.New("Не указан алиас длины (height)")}
	}
	height, err := strconv.Atoi(h)
	if err != nil || height == 0 {
		return ErrTransform{fmt.Errorf("Не верное значение алиаса длины (height) %s", h)}
	}

	//check for photos base folder (copies_1)
	basePath := path.Join(fc.wrkFolder, fmt.Sprintf("%d", item.OrderID), item.DirectoryName, "copies_1")
	if folderExists(basePath) == false {
		return errCantTransform
	}
	//check for subfolders borders and noborders
	bList, _ := fillList(path.Join(basePath, "borders"))
	nbList, _ := fillList(path.Join(basePath, "noborders"))
	if len(bList) == 0 && len(nbList) == 0 {
		//folders not exists or empty
		return errCantTransform
	}

	//copy & create order printgroup(s)/printgroupfiles
	order.PrintGroups = make([]pc.PrintGroup, 0, 2)
	outPath := path.Join(fc.cyclePrtFolder, order.FtpFolder)
	lsts := [][]fileCopy{bList, nbList}
	for i, list := range lsts {
		if len(list) == 0 {
			continue
		}
		var pg pc.PrintGroup
		if i == 0 {
			//photos with border
			pg = pc.PrintGroup{
				ID:      fmt.Sprintf("%s_%d", order.ID, len(order.PrintGroups)+1),
				OrderID: order.ID,
				Paper:   paper,
				Width:   width,
				Height:  height,
				Cutting: 20,
				Frame:   15,
				Path:    fmt.Sprintf("%d_%dx%d-%d-b", len(order.PrintGroups)+1, width, height, paper),
				State:   order.State,
			}
		} else {
			//photos without border
			pg = pc.PrintGroup{
				ID:      fmt.Sprintf("%s_%d", order.ID, len(order.PrintGroups)+1),
				OrderID: order.ID,
				Paper:   paper,
				Width:   width,
				Height:  height,
				Cutting: 19,
				Frame:   0,
				Path:    fmt.Sprintf("%d_%dx%d-%d", len(order.PrintGroups)+1, width, height, paper),
				State:   order.State,
			}
		}
		//copy to cycle print folder
		done, err := listCopy(ctx, list, path.Join(outPath, pg.Path))
		if err != nil {
			return err
		}
		if done > 0 {
			order.FotosNum = order.FotosNum + done
			pg.FileNum = done
			pg.Prints = done
			pg.Files = make([]pc.PrintGroupFile, 0, done)
			for _, fi := range list {
				if fi.Process {
					pg.Files = append(pg.Files, pc.PrintGroupFile{PrintGroupID: pg.ID, FileName: fi.NewName, Caption: fi.NewName, PrintQtty: 1})
				}
			}
			order.PrintGroups = append(order.PrintGroups, pg)
		}
	}
	//TODO check for empty order/pgs
	//create in BD move to redyToPrint state after pp state change
	return nil
}

func (fc *baseFactory) transformAlias(ctx context.Context, item pp.OrderItem, order *pc.Order) error {
	//try build by alias
	a, ok := item.Sku()["alias"]
	if !ok || a == "" {
		return errCantTransform
	}

	alias, err := fc.pcClient.LoadAlias(ctx, a)
	if err != nil {
		if err == sql.ErrNoRows {
			err = fmt.Errorf("Алиас '%s' не найден в БД", a)
		}
		return ErrTransform{err}
	}
	/*
		//TODO implement other types (magnets etc)
		switch alias.Type {
		case 1, 2, 3:
			//book
	*/
	order.HasCover = alias.HasCover
	//get file list
	basePath := path.Join(fc.wrkFolder, fmt.Sprintf("%d", item.OrderID), item.DirectoryName)
	list, err := fillList(basePath)
	if err != nil {
		return ErrSourceNotFound{err}
	}
	if len(list) == 0 {
		return ErrSourceNotFound{fmt.Errorf("Empty folder '%s'", basePath)}
	}

	err = listIndexSheets(list, alias.HasCover)
	if err != nil {
		return ErrParce{err}
	}

	//set book index
	//TODO valid only for books, recheck for other products
	if item.Quantity <= 1 {
		//one book
		for i := 0; i < len(list); i++ {
			list[i].BookIdx = 1
		}
	} else {
		//create copy of last item and set book
		fi := list[len(list)-1]
		fi.BookIdx = item.Quantity
		list = append(list, fi)
	}
	//TODO check item.PageCount, don't forget added sheet if books > 1
	//toProcess == item.PageCount

	//set output names
	//decode filenames to cycle names '000-00.jpg'
	//cover 000-00_309_5.jpg
	for i := 0; i < len(list); i++ {
		if list[i].Process {
			//width & butt for cover
			sufix := ""
			if alias.HasCover == true && list[i].SheetIdx == 0 && item.Sizes.Width > 0 {
				sufix = fmt.Sprintf("_%.0f_%.0f", math.Round(item.Sizes.Width), math.Round(item.Sizes.Thickness))
			}

			list[i].NewName = fmt.Sprintf("%03d-%02d%s%s", list[i].BookIdx, list[i].SheetIdx, sufix, filepath.Ext(list[i].OldName))
		}
	}
	//copy to cycle wrk folder/orderid/alias
	outPath := path.Join(fc.cycleFolder, order.FtpFolder)
	//clear output folder
	err = os.RemoveAll(outPath)
	if err != nil {
		return ErrFileSystem{err}
	}
	outPath = path.Join(outPath, alias.Alias)
	done, err := listCopy(ctx, list, outPath)
	if err != nil {
		return err
	}

	//update order
	order.FotosNum = done
	//?? factory has to do it
	//order.State = pc.StateConfirmation
	//order.StateDate = time.Now()
	/*
		default:
			return fmt.Errorf("Неподдерживаемый тип %d алиаса '%s'", alias.Type, alias.Alias)
		}
	*/

	return nil
}

//FromPPOrder converts PP order to photocycle order
func fromPPOrder(o pp.Order, source int, sufix string) pc.Order {
	g, err := strconv.Atoi(o.ID)
	if err != nil {
		g = 0
	}
	return pc.Order{
		ID:         fmt.Sprintf("%d_%s%s", source, o.ID, sufix),
		Source:     source,
		SourceID:   o.ID,
		SourceDate: time.Time(o.DateCreated),
		DataTS:     time.Time(o.DateModified),
		GroupID:    g,
		ClientID:   o.UserID, //??
		FtpFolder:  fmt.Sprintf("%s%s", o.ID, sufix),
	}
}

func buildExtraInfo(forOrder pc.Order, from pp.OrderItem) pc.OrderExtraInfo {
	sheets := from.PageCount
	if forOrder.HasCover {
		sheets = sheets - 1
	}

	return pc.OrderExtraInfo{
		ID:      forOrder.ID,
		GroupID: forOrder.GroupID,
		Format:  from.Name,
		Books:   from.Quantity,
		Sheets:  sheets,
		Alias:   from.Sku()["alias"],
		Paper:   from.Sku()["paper"],
		Remark:  from.Comment,
		Title:   from.Description,
		Date:    forOrder.SourceDate,
	}
}

type fileCopy struct {
	OldPath  string
	OldName  string
	NewName  string
	Process  bool
	SheetIdx int
	BookIdx  int
}

func fillList(path string) ([]fileCopy, error) {

	f, err := folderOpen(path)
	if err != nil {
		return []fileCopy{}, err
	}
	defer f.Close()

	list, err := f.Readdir(-1)
	if err != nil {
		return []fileCopy{}, err
	}

	var res = make([]fileCopy, 0, len(list))
	for _, fi := range list {
		if !fi.IsDir() {
			res = append(res, fileCopy{OldPath: path, OldName: fi.Name(), NewName: fi.Name(), Process: allowedExt[filepath.Ext(fi.Name())]})
		}
	}
	return res, nil
}

func listIndexSheets(list []fileCopy, hasCover bool) error {
	rep, err := regexp.Compile(`(_preview\.)`)
	if err != nil {
		return err
	}
	//fmt.Println(re.MatchString("surface_0_preview.png"))
	rei, err := regexp.Compile(`^surface_\[(\d+)\]`)
	if err != nil {
		return err
	}
	//m :=re.FindStringSubmatch("surface_[78888](oblozhka)_zone_[0](oblozhka).jpg")

	for i, fi := range list {
		if fi.Process {
			if rep.MatchString(fi.OldName) {
				//exclude preview
				list[i].Process = false
			} else {
				//get surface index
				sm := rei.FindStringSubmatch(fi.OldName)
				if len(sm) != 2 {
					//TODO error?
					list[i].Process = false
				} else {
					idx, err := strconv.Atoi(sm[1])
					if err != nil {
						return err
					}
					if !hasCover {
						idx++
					}
					list[i].SheetIdx = idx
				}
			}
		}
	}
	return nil
}

func listCopy(ctx context.Context, list []fileCopy, toFolder string) (done int, err error) {
	//clear output folder

	if err = recreateFolder(toFolder); err != nil {
		return 0, ErrFileSystem{err}
	}
	//copy to cycle print folder
	//copy files
	for _, fi := range list {
		//check if transform context is canceled
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			// keep working
		}
		if fi.Process {
			if err = copyFile(path.Join(fi.OldPath, fi.OldName), path.Join(toFolder, fi.NewName)); err != nil {
				return 0, ErrFileSystem{err}
			}
			done++
		}
	}
	return
}
