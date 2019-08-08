package transform

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/cavaliercoder/grab"
	pc "github.com/egorka-gh/pixlpark/photocycle"
	pp "github.com/egorka-gh/pixlpark/pixlpark/service"
)

// Factory is factory of transform item (Transform)
// creates transform item and defines transform process
type Factory struct {
	ppClient  pp.PPService
	pcClient  pc.Repository
	source    int
	wrkFolder string
	resFolder string
	ppUser    string
}

// NewFactory returns a new transform Factory, using provided configuration.
func NewFactory(pixlparkClient pp.PPService, photocycleClient pc.Repository, sourse int, workFolder, resultFolder, pixlparkUserEmail string) *Factory {
	return &Factory{
		ppClient:  pixlparkClient,
		pcClient:  photocycleClient,
		source:    sourse,
		wrkFolder: workFolder,
		resFolder: resultFolder,
		ppUser:    pixlparkUserEmail,
	}
}

// Do main sequence
// fetch new order and perfom full trunsform
//
// Like http.Get, Do blocks while the trunsform is initiated, but returns as soon
// as the trunsform has started  in a background goroutine, or if it
// failed early.
// If no order fetched returns nil Transform
//
// An error is returned via Transform.Err. Transform.Err
// will block the caller until the transform is completed, successfully or
// otherwise.
func (fc *Factory) Do(ctx context.Context) *Transform {
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
	fc.run(t, fc.fetchToLoad)
	//TODO maybe transform manager has to now wich kinde of error?
	if t.IsComplete() {
		//TODO log error
		return nil
	}
	//Run load in a new goroutine
	go fc.run(t, fc.loadZIP)
	return t
}

// An stateFunc is an action that mutates the state of a Transform and returns
// the next stateFunc to be called.
type stateFunc func(*Transform) stateFunc

// run calls the given stateFunc function and all subsequent returned stateFuncs
// until a stateFunc returns nil or the Transform.ctx is canceled. Each stateFunc
// should mutate the state of the given Transform until it has completed or failed.
func (fc *Factory) run(t *Transform, f stateFunc) {
	for {
		select {
		case <-t.ctx.Done():
			if t.IsComplete() {
				return
			}
			t.err = t.ctx.Err()
			f = fc.closeTransform

		default:
			// keep working
		}
		if f = f(t); f == nil {
			return
		}
	}
}

func (fc *Factory) fetchToLoad(t *Transform) stateFunc {
	//pp.StatePrepressCoordination
	orders, err := fc.ppClient.GetOrders(t.ctx, pp.StateReadyToProcessing, 0, 0, 10, 0)
	if err != nil {
		t.err = err
		return fc.closeTransform
	}

	for _, po := range orders {
		co := pc.FromPPOrder(po, fc.source, "@")
		co.State = pc.StateLoadWaite
		co, _ = fc.pcClient.CreateOrder(t.ctx, co)
		//TODO load/check state from all orders by group?
		if co.State == pc.StateCanceledPHCycle {
			//cancel in pp
			//ignore errors
			fc.ppClient.AddOrderComment(t.ctx, po.ID, fc.ppUser, "Заказ отменен в PhotoCycle")
			fc.ppClient.SetOrderStatus(t.ctx, po.ID, pp.StatePrintedWithDefect, false)
			continue
		}
		err := fc.ppClient.SetOrderStatus(t.ctx, po.ID, pp.StatePrepressCoordination, false)
		if err != nil {
			continue
		}
		//сan load stop fetching
		t.ppOrder = po
		t.pcBaseOrder = co
		return nil
	}
	t.err = ErrEmptyQueue(fmt.Errorf("No orders in state %s", pp.StateReadyToProcessing))
	return fc.closeTransform
}

//start grab to load zip
func (fc *Factory) loadZIP(t *Transform) stateFunc {
	loader := grab.NewClient()
	fl := filepath.Join(fc.wrkFolder, t.ppOrder.ID+".zip")
	//check delete old zip & folder
	if err := os.Remove(fl); !os.IsNotExist(err) {
		t.err = err
		return fc.closeTransform
	}
	if err := os.RemoveAll(path.Join(fc.wrkFolder, t.ppOrder.ID)); !os.IsNotExist(err) {
		t.err = err
		return fc.closeTransform
	}

	req, err := grab.NewRequest(fl, t.ppOrder.DownloadLink)
	req = req.WithContext(t.ctx)
	req.SkipExisting = true
	req.NoResume = true
	if err != nil {
		t.err = err
		return fc.closeTransform
	}
	t.loader = loader.Do(req)
	//waite till complete
	err = t.loader.Err()
	if err != nil {
		//TODO err limiter
		t.err = err
		return fc.closeTransform
	}

	//TODO forvard to unzip
	//dummy stop
	return fc.closeTransform
}

// closeTransform finalizes the Transform
func (fc *Factory) closeTransform(t *Transform) stateFunc {
	if t.IsComplete() {
		panic("transform: developer error: transform already closed")
	}

	//TODO release resources
	/*
		resp.fi = nil
		closeWriter(resp)
		resp.closeResponseBody()
	*/
	if t.loader != nil {
		if !t.loader.IsComplete() {
			//blocks till cancel
			t.loader.Cancel()
		}
		t.loader = nil
	}

	t.End = time.Now()
	close(t.Done)
	if t.cancel != nil {
		t.cancel()
	}

	return nil
}

//ErrEmptyQueue - no orders to transform
type ErrEmptyQueue error
