package transform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"

	pc "github.com/egorka-gh/pixlpark/photocycle"
	pp "github.com/egorka-gh/pixlpark/pixlpark/service"
)

//internal errors, factory has to make decision what to do (rise error or proceed transform)

//ErrCantTransform inapplicable transform method
type ErrCantTransform error

//ErrSourceNotFound source file or folder not found
type ErrSourceNotFound error

//ErrParce parce error (filename or some else text field)
type ErrParce error

//ErrFileSystem file system error
type ErrFileSystem error

var (
	errCantTransform = ErrCantTransform(errors.New("Can't transform"))
)

type fileCopy struct {
	OldName  string
	NewName  string
	Process  bool
	SheetIdx int
	BookIdx  int
}

func (fc *Factory) transformAlias(ctx context.Context, item pp.OrderItem, order *pc.Order) error {
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
		return err
	}
	//TODO implement other types (magnets etc)
	switch alias.Type {
	case 1, 2, 3:
		//book
		//get file list
		basePath := path.Join(fc.wrkFolder, fmt.Sprintf("%d", item.OrderID), item.DirectoryName)
		list, err := folderList(basePath)
		if err != nil {
			return ErrSourceNotFound(err)
		}
		if len(list) == 0 {
			return ErrSourceNotFound(fmt.Errorf("Empty folder '%s'", basePath))
		}

		err = listIndexItems(list, alias.HasCover)
		if err != nil {
			return ErrParce(err)
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
		toProcess := 0
		for i := 0; i < len(list); i++ {
			if list[i].Process {
				list[i].NewName = fmt.Sprintf("%03d-%02d%s", list[i].BookIdx, list[i].SheetIdx, filepath.Ext(list[i].OldName))
				toProcess++
			} else {
				list[i].NewName = list[i].OldName
			}
		}
		//copy to cycle wrk folder/orderid/alias
		outPath := path.Join(fc.cycleFolder, order.FtpFolder)
		//clear output folder
		err = os.RemoveAll(outPath)
		if err != nil {
			return ErrFileSystem(err)
		}
		err = os.MkdirAll(outPath, 0755)
		if err != nil {
			return ErrFileSystem(err)
		}
		//copy files
		for _, fi := range list {
			//check if transform context is canceled
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				// keep working
			}
			//TODO copy only vs Process ??
			//if list[i].Process {
			err = copyFile(path.Join(basePath, fi.OldName), path.Join(outPath, fi.NewName))
			if err != nil {
				return ErrFileSystem(err)
			}
		}

		//update order
		order.FotosNum = toProcess
		//?? factory has to do it
		order.State = pc.StateConfirmation
		//order.StateDate = time.Now()
	default:
		return fmt.Errorf("Неподдерживаемый тип %d алиаса '%s'", alias.Type, alias.Alias)
	}

	return nil
}

func folderList(path string) ([]fileCopy, error) {

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
			res = append(res, fileCopy{OldName: fi.Name(), Process: allowedExt[filepath.Ext(fi.Name())]})
		}
	}
	return res, nil
}

func listIndexItems(list []fileCopy, hasCover bool) error {
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
