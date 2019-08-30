package transform

import (
	"context"
	"fmt"
	"time"

	pc "github.com/egorka-gh/pixlpark/photocycle"
	pp "github.com/egorka-gh/pixlpark/pixlpark/service"
)

// DoOrder loads order and perfom full trunsform (4 tests only)
//
// Like DO, DoOrder blocks while the trunsform is initiated, but returns as soon
// as the trunsform has started  in a background goroutine, or if it
// failed early.
//
// An error is returned via Transform.Err. Transform.Err
// will block the caller until the transform is completed, successfully or
// otherwise.
func (fc *Factory) DoOrder(ctx context.Context, id string) *Transform {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	t := &Transform{
		Start:   time.Now(),
		Done:    make(chan struct{}, 0),
		ctx:     ctx,
		cancel:  cancel,
		ppOrder: pp.Order{ID: id},
	}

	// Run state-machine while caller is blocked to fetch pixelpark order and to initialize transform.
	fc.run(t, fc.getOrder)

	if t.IsComplete() {
		fc.log("DoOrder Error", t.Err().Error())
		return t
	}
	//Run load in a new goroutine
	go fc.run(t, fc.loadZIP)
	return t
}

//ResetStarted resets orders in state statePixelLoadStarted to state statePixelStartLoad (4 debug only)
func (fc *Factory) ResetStarted(ctx context.Context) *Transform {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	t := &Transform{
		Start:  time.Now(),
		Done:   make(chan struct{}, 0),
		ctx:    ctx,
		cancel: cancel,
	}

	// Run state-machine while caller is blocked to fetch pixelpark order and to initialize transform.
	fc.run(t, fc.resetFetched)

	if !t.IsComplete() {
		panic("transform: developer error: resetFetched must return completed transform")
	}
	fc.log("ResetStarted", t.Err().Error())
	return nil
}

//getOrder tries to load order from PP (4 tests only)
//expects ID in dummy t.ppOrder
//on success t is not closed (valid for processing)
//TODO 4 production add states check
func (fc *Factory) getOrder(t *Transform) stateFunc {
	fc.log("getOrder", t.ppOrder.ID)
	po, err := fc.ppClient.GetOrder(t.ctx, t.ppOrder.ID)
	if err != nil {
		t.err = ErrService{err}
		return fc.closeTransform
	}
	co := fromPPOrder(po, fc.source, "@")
	co.State = pc.StateLoadWaite
	_ = fc.pcClient.CreateOrder(t.ctx, co)
	_ = fc.pcClient.SetOrderState(t.ctx, co.ID, co.State)

	if !fc.Debug {
		if err = fc.ppClient.SetOrderStatus(t.ctx, po.ID, statePixelLoadStarted, false); err != nil {
			t.err = ErrService{err}
			return fc.closeTransform
		}
	}
	//loaded
	t.ppOrder = po
	t.pcBaseOrder = co
	return nil
}

//resetFetched - reset orders in statePixelLoadStarted (4 dubug only)
func (fc *Factory) resetFetched(t *Transform) stateFunc {
	doFetch := true
	skip := 0
	//while has some orders in statePixelLoadStarted
	//can be Infinite loop if some SetOrderStatus incompleted ??
	for doFetch {
		orders, err := fc.ppClient.GetOrders(t.ctx, statePixelLoadStarted, 0, 0, 10, skip)

		if err != nil {
			t.err = err
			return fc.closeTransform
		}
		doFetch = len(orders) > 0
		for _, po := range orders {
			fc.log("resetFetched", po.ID)
			co := fromPPOrder(po, fc.source, "@")

			//TODO load/check state from all orders by group?
			if co.State == pc.StateCanceledPHCycle {
				//cancel in pp
				if err := fc.setPixelState(t, statePixelAbort, "Заказ отменен в PhotoCycle"); err != nil {
					skip++
				}
				continue
			}
			//reset
			if err := fc.ppClient.SetOrderStatus(t.ctx, po.ID, statePixelStartLoad, false); err != nil {
				skip++
				continue
			}
			_ = fc.setCycleState(t, pc.StateLoadWaite, 0, "")
		}
	}

	t.err = ErrEmptyQueue{fmt.Errorf("No orders in state %s", statePixelLoadStarted)}
	return fc.closeTransform
}
