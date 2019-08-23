package transform

import (
	"archive/zip"
	"context"
	"errors"
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
	ppClient    pp.PPService
	pcClient    pc.Repository
	source      int
	wrkFolder   string
	cycleFolder string
	ppUser      string
	logger      log.Logger
}

//internal errors, factory has to make decision what to do (rise error or proceed transform)

//ErrEmptyQueue - no orders to transform
type ErrEmptyQueue error

//ErrCantTransform inapplicable transform method
type ErrCantTransform error

//ErrSourceNotFound source file or folder not found
type ErrSourceNotFound error

//ErrParce parce error (filename or some else text field)
type ErrParce error

//ErrFileSystem file system error
type ErrFileSystem error

//ErrRepository lokal data base error
type ErrRepository error

//ErrService PP servise error
type ErrService error

var (
	errCantTransform = ErrCantTransform(errors.New("Can't transform"))
)

// NewFactory returns a new transform Factory, using provided configuration.
func NewFactory(pixlparkClient pp.PPService, photocycleClient pc.Repository, sourse int, workFolder, resultFolder, pixlparkUserEmail string, logger log.Logger) *Factory {
	return &Factory{
		ppClient:    pixlparkClient,
		pcClient:    photocycleClient,
		source:      sourse,
		wrkFolder:   workFolder,
		cycleFolder: resultFolder,
		ppUser:      pixlparkUserEmail,
		logger:      logger,
	}
}

// Do main sequence
// fetch new order and perfom full trunsform
//
// Like http.Get, Do blocks while the trunsform is initiated, but returns as soon
// as the trunsform has started  in a background goroutine, or if it
// failed early.
// If no order fetched returns error ErrEmptyQueue and closed transform
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
		return t
	}
	//Run load in a new goroutine
	go fc.run(t, fc.loadZIP)
	return t
}

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

// An stateFunc is factory unit of work
// is an action that mutates the state of a Transform and returns
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

//fetchToLoad looks for in PP the next order to process
//bloks till get some order to process
//on success t is not closed (valid for processing)
//if there is no orders returns ErrEmptyQueue
func (fc *Factory) fetchToLoad(t *Transform) stateFunc {
	//pp.StatePrepressCoordination
	orders, err := fc.ppClient.GetOrders(t.ctx, statePixelStartLoad, 0, 0, 10, 0)
	if err != nil {
		t.err = err
		return fc.closeTransform
	}

	for _, po := range orders {
		fc.log("fetch", po.ID)
		co := fromPPOrder(po, fc.source, "@")
		co.State = pc.StateLoadWaite
		_ = fc.pcClient.CreateOrder(t.ctx, co)
		co, _ = fc.pcClient.LoadOrder(t.ctx, co.ID)
		//TODO load/check state from all orders by group?
		if co.State == pc.StateCanceledPHCycle {
			//cancel in pp
			//ignore errors
			fc.setPixelState(t, statePixelAbort, "Заказ отменен в PhotoCycle")
			//keep fetching
			continue
		}

		if err = fc.ppClient.SetOrderStatus(t.ctx, po.ID, statePixelLoadStarted, false); err != nil {
			//keep fetching
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

//getOrder tries to load order from PP (4 tests only)
//expects ID in dummy t.ppOrder
//on success t is not closed (valid for processing)
//TODO 4 production add states check
func (fc *Factory) getOrder(t *Transform) stateFunc {
	fc.log("getOrder", t.ppOrder.ID)
	po, err := fc.ppClient.GetOrder(t.ctx, t.ppOrder.ID)
	if err != nil {
		t.err = ErrService(err)
		return fc.closeTransform
	}
	co := fromPPOrder(po, fc.source, "@")
	co.State = pc.StateLoadWaite
	_ = fc.pcClient.CreateOrder(t.ctx, co)
	fc.pcClient.SetOrderState(t.ctx, co.ID, co.State)

	if err = fc.ppClient.SetOrderStatus(t.ctx, po.ID, statePixelLoadStarted, false); err != nil {
		t.err = ErrService(err)
		return fc.closeTransform
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
	return fc.transformItems
}

//transformItems transforms orderitems to cycle orders
func (fc *Factory) transformItems(t *Transform) stateFunc {
	var err error
	fc.log("transformItems", t.ppOrder.ID)

	//TODO  don't need store in t.ppOrder.Items
	items, err := fc.ppClient.GetOrderItems(t.ctx, t.ppOrder.ID)
	if err != nil {
		//TODO restart?
		t.err = err
		fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
		fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrFileSystem, t.err.Error())
		return fc.closeTransform
	}

	//TODO check if cycle allready print suborder??
	//TODO  don't need store in t.pcOrders
	//t.pcOrders = make([]pc.Order, 0, len(t.ppOrder.Items))
	orders := make([]pc.Order, 0, len(items))
	incomlete := false
	for i, item := range items {
		//process item
		fc.log("item", fmt.Sprintf("%s-%d", t.ppOrder.ID, item.ID))
		//create cycle order
		co := fromPPOrder(t.ppOrder, fc.source, fmt.Sprintf("-%d", i))
		co.SourceID = fmt.Sprintf("%s-%d", t.ppOrder.ID, item.ID)
		//try build by alias
		alias := item.Sku()["alias"]
		if alias != "" {
			err = fc.transformAlias(t.ctx, item, &co)
			if err != nil {
				incomlete = true
				msg := fmt.Sprintf("Элемент заказа %s '%s' не обработан. Ошибка %s", co.SourceID, item.Name, err.Error())
				fc.log("error", msg)
				fc.setPixelState(t, "", msg)
				fc.setCycleState(t, 0, pc.StateErrPreprocess, msg)
			} else {
				co.State = pc.StateConfirmation
				orders = append(orders, co)
			}
		} else {
			//TODO some other
			incomlete = true
			msg := fmt.Sprintf("Элемент заказа %s '%s' не обработан. Для продукта не настроены параметры подготовки", co.SourceID, item.Name)
			fc.log("error", msg)
			fc.setPixelState(t, "", msg)
			fc.setCycleState(t, 0, pc.StateErrPreprocess, msg)
		}
	}
	if incomlete {
		msg := "Часть элементов заказа не обработано. Заказ не размещен в Photocycle"
		fc.log("error", msg)
		fc.setPixelState(t, statePixelWaiteConfirm, msg)
		fc.setCycleState(t, pc.StatePreprocessIncomplite, pc.StateErrPreprocess, msg)
	} else {
		//clear sub orders
		err = fc.pcClient.ClearGroup(t.ctx, t.pcBaseOrder.GroupID, t.pcBaseOrder.ID)
		if err != nil {
			t.err = ErrRepository(err)
			fc.setPixelState(t, statePixelWaiteConfirm, fmt.Sprintf("Ошибка БД: %s", err.Error()))
			fc.setCycleState(t, pc.StatePreprocessIncomplite, pc.StateErrWrite, err.Error())
			return fc.closeTransform
		}
		//create sub orders in  pc.StateConfirmation
		for _, co := range orders {
			err = fc.pcClient.CreateOrder(t.ctx, co)
			if err != nil {
				t.err = ErrRepository(err)
				fc.setPixelState(t, statePixelWaiteConfirm, fmt.Sprintf("Ошибка БД: %s", err.Error()))
				fc.setCycleState(t, pc.StatePreprocessIncomplite, pc.StateErrWrite, err.Error())
				return fc.closeTransform
			}
		}

		//finalase
		//TODO check production
		//set state in pp
		err = fc.ppClient.SetOrderStatus(t.ctx, t.ppOrder.ID, pp.StatePrinting, true)
		if err != nil {
			//TODO report err??
			t.err = err
			//fc.setPixelState(t, statePixelWaiteConfirm, fmt.Sprintf("Ошибка БД: %s", err.Error()))
			fc.setCycleState(t, pc.StatePreprocessIncomplite, pc.StateErrWeb, fmt.Sprintf("Ошибка смены статуса на сайте. Статус:%s; Ошибка:%s ", pp.StatePrinting, err.Error()))
			return fc.closeTransform
		}
		//move all to waite print state
		err = fc.pcClient.SetGroupState(t.ctx, pc.StatePreprocessWaite, t.pcBaseOrder.GroupID, t.pcBaseOrder.ID)
		if err != nil {
			//TODO report err??
			t.err = ErrRepository(err)
			fc.setPixelState(t, "", fmt.Sprintf("Ошибка БД: %s", err.Error()))
			fc.setCycleState(t, pc.StatePreprocessIncomplite, pc.StateErrWrite, err.Error())
			return fc.closeTransform
		}

		//move main to complite state
		fc.setCycleState(t, pc.StateSkiped, 0, "")
		//TODO finalize
		//kill zip and unziped folder
	}

	//TODO forvard to ?
	//stop
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
