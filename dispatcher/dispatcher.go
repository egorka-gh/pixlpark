package dispatcher

import (
	"context"
	"errors"
	"sync/atomic"

	pc "github.com/egorka-gh/pixlpark/photocycle"
	pp "github.com/egorka-gh/pixlpark/pixlpark/service"
)

//Dispatcher directs orders flow between pixelpark and photocycle
type dispatcher struct {
	ppClient  pp.PPService
	pcClient  pc.Repository
	source    int
	wrkFolder string
	resFolder string
	isLoading int64
	ppUser    string
}

type item struct {
	ppOrder     pp.Order
	pcBaseOrder pc.Order
	pcOrders    []pc.Order
}

//Run start loading queue and sync pp vs cycle
func (d *dispatcher) Run() {
	if atomic.CompareAndSwapInt64(&d.isLoading, 0, 1) {
		//start loading
	}
	//sync cycle state 199 vs pp.printing set in cycle 200
}

func (d *dispatcher) fetchToLoad(ctx context.Context) (*item, error) {
	//pp.StatePrepressCoordination
	orders, err := d.ppClient.GetOrders(ctx, pp.StateReadyToProcessing, 0, 0, 10, 0)
	if err != nil {
		return nil, err
	}

	for _, po := range orders {
		co := pc.FromPPOrder(po, d.source, "@")
		co.State = pc.StateLoadWaite
		co, _ = d.pcClient.CreateOrder(ctx, co)
		if co.State == pc.StateCanceledPHCycle {
			//cancel in pp
			//ignore errors
			d.ppClient.AddOrderComment(ctx, po.ID, d.ppUser, "Заказ отменен в PhotoCycle")
			d.ppClient.SetOrderStatus(ctx, po.ID, pp.StatePrintedWithDefect, false)
			continue
		}
		err := d.ppClient.SetOrderStatus(ctx, po.ID, pp.StatePrepressCoordination, false)
		if err != nil {
			continue
		}
		return &item{ppOrder: po, pcBaseOrder: co}, nil
	}
	return nil, ErrAbort(errors.New("No orders to load"))
}

func (d *dispatcher) load(ctx context.Context, item *item) (*item, error) {

	return item, nil
}

//ErrAbort - abort processing
type ErrAbort error
