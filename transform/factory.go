package transform

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	log "github.com/go-kit/kit/log"

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
	logger    log.Logger
}

// NewFactory returns a new transform Factory, using provided configuration.
func NewFactory(pixlparkClient pp.PPService, photocycleClient pc.Repository, sourse int, workFolder, resultFolder, pixlparkUserEmail string, logger log.Logger) *Factory {
	return &Factory{
		ppClient:  pixlparkClient,
		pcClient:  photocycleClient,
		source:    sourse,
		wrkFolder: workFolder,
		resFolder: resultFolder,
		ppUser:    pixlparkUserEmail,
		logger:    logger,
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
		fc.log("Do Error", t.Err().Error())
		return nil
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

func (fc *Factory) log(key, massage string) {
	if fc.logger != nil {
		fc.logger.Log(key, massage)
	}
}

func (fc *Factory) fetchToLoad(t *Transform) stateFunc {
	//pp.StatePrepressCoordination
	orders, err := fc.ppClient.GetOrders(t.ctx, statePixelStartLoad, 0, 0, 10, 0)
	if err != nil {
		t.err = err
		return fc.closeTransform
	}

	for _, po := range orders {
		fc.log("fetch", po.ID)
		co := pc.FromPPOrder(po, fc.source, "@")
		co.State = pc.StateLoadWaite
		co, _ = fc.pcClient.CreateOrder(t.ctx, co)
		//TODO load/check state from all orders by group?
		if co.State == pc.StateCanceledPHCycle {
			//cancel in pp
			//ignore errors
			fc.setPixelState(t, statePixelAbort, "Заказ отменен в PhotoCycle")
			continue
		}
		err := fc.ppClient.SetOrderStatus(t.ctx, po.ID, statePixelLoadStarted, false)
		if err != nil {
			continue
		}
		//сan load stop fetching
		t.ppOrder = po
		t.pcBaseOrder = co
		return nil
	}
	t.err = ErrEmptyQueue(fmt.Errorf("No orders in state %s", statePixelStartLoad))
	return fc.closeTransform
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
			co := pc.FromPPOrder(po, fc.source, "@")

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
			fc.setCycleState(t, pc.StateLoadWaite, 0, "")
		}
	}

	t.err = ErrEmptyQueue(fmt.Errorf("No orders in state %s", statePixelLoadStarted))
	return fc.closeTransform
}

//start grab client to load zip
func (fc *Factory) loadZIP(t *Transform) stateFunc {
	fc.log("loadZIP", t.ppOrder.ID)

	loader := grab.NewClient()
	fl := filepath.Join(fc.wrkFolder, t.ppOrder.ID+".zip")
	//check delete old zip & folder
	if err := os.Remove(fl); err != nil && !os.IsNotExist(err) {
		t.err = err
	}
	if err := os.RemoveAll(path.Join(fc.wrkFolder, t.ppOrder.ID)); err != nil && !os.IsNotExist(err) {
		t.err = err
	}
	if t.err != nil {
		//filesystem err
		//reset && close
		fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки:"+t.err.Error())
		fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrFileSystem, t.err.Error())
		return fc.closeTransform
	}

	req, err := grab.NewRequest(fl, t.ppOrder.DownloadLink)
	if err != nil {
		t.err = err
		fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
		fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrWeb, t.err.Error())
		return fc.closeTransform
	}
	req = req.WithContext(t.ctx)
	req.SkipExisting = true
	req.NoResume = true
	fc.setCycleState(t, 0, pc.StateLoad, "")
	t.loader = loader.Do(req)
	//waite till complete
	t.err = t.loader.Err()
	if t.err != nil {
		//TODO err limiter
		fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
		fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrWeb, t.err.Error())
		return fc.closeTransform
	}

	//forvard to unzip
	return fc.unzip
}

//unpack zip
func (fc *Factory) unzip(t *Transform) stateFunc {
	//TODO err limiter??
	fc.log("unzip", t.ppOrder.ID)
	fc.setCycleState(t, 0, pc.StateUnzip, "")
	reader, err := zip.OpenReader(filepath.Join(fc.wrkFolder, t.ppOrder.ID+".zip"))
	if err != nil {
		t.err = err
		fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
		fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrZip, t.err.Error())
		return fc.closeTransform
	}
	basePath := path.Join(fc.wrkFolder, t.ppOrder.ID)
	if err = os.MkdirAll(basePath, 0755); err != nil {
		t.err = err
		fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
		fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrFileSystem, t.err.Error())
		return fc.closeTransform
	}

	for _, file := range reader.File {
		//check if transform context is canceled
		select {
		case <-t.ctx.Done():
			if t.IsComplete() {
				return nil
			}
			t.err = t.ctx.Err()
			return fc.closeTransform
		default:
			// keep working
		}

		//expected that zip has 1 root folder vs sub folder for each order item
		filePpath := replaceRootPath(file.Name, basePath) //filepath.Join(basePath, file.Name)

		if file.FileInfo().IsDir() {
			os.MkdirAll(filePpath, file.Mode())
			continue
		}

		//create folder 4 file
		if err := os.MkdirAll(filepath.Dir(filePpath), 0755); err != nil {
			t.err = err
			fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
			fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrFileSystem, t.err.Error())
			return fc.closeTransform
		}

		//open in zip
		fileReader, err := file.Open()
		if err != nil {
			t.err = err
			fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
			fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrZip, t.err.Error())
			return fc.closeTransform
		}
		defer fileReader.Close()

		//create in filesystem
		targetFile, err := os.OpenFile(filePpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			t.err = err
			fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
			fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrFileSystem, t.err.Error())
			return fc.closeTransform
		}
		defer targetFile.Close()

		//extract
		if _, err := io.Copy(targetFile, fileReader); err != nil {
			t.err = err
			fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
			fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrZip, t.err.Error())
			return fc.closeTransform
		}
	}

	//TODO forvard to transform
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

func (fc *Factory) setPixelState(t *Transform, state, message string) error {
	var err error
	if state != "" {
		err = fc.ppClient.SetOrderStatus(t.ctx, t.ppOrder.ID, state, false)
	}
	if message != "" {
		fc.ppClient.AddOrderComment(t.ctx, t.ppOrder.ID, fc.ppUser, message)
	}
	return err
}

func (fc *Factory) setCycleState(t *Transform, state int, logState int, message string) error {
	var err error
	if state != 0 {
		err = fc.pcClient.SetOrderState(t.ctx, t.pcBaseOrder.ID, state)
	}
	if message != "" {
		fc.pcClient.LogState(t.ctx, t.pcBaseOrder.ID, logState, message)
	}
	return err
}

//ErrEmptyQueue - no orders to transform
type ErrEmptyQueue error
