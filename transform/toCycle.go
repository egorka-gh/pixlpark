package transform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"

	pc "github.com/egorka-gh/pixlpark/photocycle"
	pp "github.com/egorka-gh/pixlpark/pixlpark/service"
)

//internal errors, factory has to make decision what to do (rise error or proceed transform)

//ErrCantTransform inapplicable transform method
type ErrCantTransform error

//ErrSourceNotFound source file or folder not found
type ErrSourceNotFound error

var (
	errCantTransform = ErrCantTransform(errors.New("Can't transform"))
)

type fileCopy struct {
	OldName  string
	NewName  string
	Process  bool
	SheetIdx int
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

		//exclude preview
		/*
			re := regexp.MustCompile(`(_preview\.)`)
			fmt.Println(re.MatchString("surface_0_preview.png"))
		*/
		//get index
		/*
			re := regexp.MustCompile(`^surface_\[(\d+)\]`)
			m :=re.FindStringSubmatch("surface_[78888](oblozhka)_zone_[0](oblozhka).jpg")
		*/

		//decode filenames to cycle names '000-00.jpg'
		//copy to wrk folder/orderid/alias
		//check books num create copy of some file vs last book num
		//update order
	default:
		return fmt.Errorf("Неподдерживаемый тип %d алиаса '%s'", alias.Type, alias.Alias)
	}
	//dummy return
	return nil
}
