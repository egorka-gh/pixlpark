package transform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	pc "github.com/egorka-gh/pixlpark/photocycle"
	pp "github.com/egorka-gh/pixlpark/pixlpark/service"
)

var errCantTransform = errors.New("CantTransform")

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
