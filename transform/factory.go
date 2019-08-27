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
type ErrEmptyQueue struct {
	error
}

//ErrCantTransform inapplicable transform method
type ErrCantTransform struct {
	error
}

//ErrTransform transform error
type ErrTransform struct {
	error
}

//ErrSourceNotFound source file or folder not found
type ErrSourceNotFound struct {
	error
}

//ErrParce parce error (filename or some else text field)
type ErrParce struct {
	error
}

//ErrFileSystem file system error
type ErrFileSystem struct {
	error
}

//ErrRepository lokal data base error
type ErrRepository struct {
	error
}

//ErrService PP servise error
type ErrService struct {
	error
}

var (
	errCantTransform = ErrCantTransform{errors.New("Can't transform")}
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
//fetch in PP state statePixelStartLoad
//fetched order moves to statePixelLoadStarted in PP and to StateLoadWaite in cycle
//must check order state in cycle (not implemented)
func (fc *Factory) fetchToLoad(t *Transform) stateFunc {
	//pp.StatePrepressCoordination
	orders, err := fc.ppClient.GetOrders(t.ctx, statePixelStartLoad, 0, 0, 10, 0)
	if err != nil {
		t.err = ErrService{err}
		return fc.closeTransform
	}

	for _, po := range orders {
		fc.log("fetch", po.ID)
		co := fromPPOrder(po, fc.source, "@")
		co.State = pc.StateLoadWaite
		//check states in cycle
		//load/check state from all orders by group
		states, err := fc.pcClient.GetGroupState(t.ctx, co.ID, co.GroupID)
		if err != nil {
			//TODO database not responding??
			err = ErrRepository{err}
			//stop all?
			continue
		}
		if states.BaseState == 0 && states.ChildState == 0 {
			//normal flow - create base
			if err = fc.pcClient.CreateOrder(t.ctx, co); err != nil {
				err = ErrRepository{err}
				continue
			}
		} else {
			if states.BaseState == pc.StateCanceledPHCycle {
				//cancel in pp
				_ = fc.setPixelState(t, statePixelAbort, "Заказ отменен в PhotoCycle")
				continue
			}
			if states.ChildState > pc.StatePrintWaite {
				//cancel in pp
				_ = fc.setPixelState(t, statePixelAbort, "Заказ отправлен на печать в PhotoCycle")
				continue
			}
			//clear sub orders
			if states.ChildState > 0 {
				_ = fc.pcClient.ClearGroup(t.ctx, co.GroupID, co.ID)
			}
			//reset base state
			if states.BaseState != co.State {
				_ = fc.pcClient.SetOrderState(t.ctx, co.ID, co.State)
			}
		}
		if err = fc.ppClient.SetOrderStatus(t.ctx, po.ID, statePixelLoadStarted, false); err != nil {
			//keep fetching
			err = ErrService{err}
			continue
		}
		//сan load stop fetching
		t.ppOrder = po
		t.pcBaseOrder = co
		return nil
	}
	if err != nil {
		//return last error
		t.err = err
	} else {
		//empty queue
		t.err = ErrEmptyQueue{fmt.Errorf("No orders in state %s", statePixelStartLoad)}
	}
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
		t.err = ErrService{err}
		return fc.closeTransform
	}
	co := fromPPOrder(po, fc.source, "@")
	co.State = pc.StateLoadWaite
	_ = fc.pcClient.CreateOrder(t.ctx, co)
	_ = fc.pcClient.SetOrderState(t.ctx, co.ID, co.State)

	if err = fc.ppClient.SetOrderStatus(t.ctx, po.ID, statePixelLoadStarted, false); err != nil {
		t.err = ErrService{err}
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
			_ = fc.setCycleState(t, pc.StateLoadWaite, 0, "")
		}
	}

	t.err = ErrEmptyQueue{fmt.Errorf("No orders in state %s", statePixelLoadStarted)}
	return fc.closeTransform
}

//start grab client to load zip
//in/out states statePixelLoadStarted in PP and StateLoadWaite in cycle
func (fc *Factory) loadZIP(t *Transform) stateFunc {
	fc.log("loadZIP", t.ppOrder.ID)

	loader := grab.NewClient()
	fl := filepath.Join(fc.wrkFolder, t.ppOrder.ID+".zip")
	//check delete old zip & folder
	if err := os.Remove(fl); err != nil && !os.IsNotExist(err) {
		t.err = ErrFileSystem{err}
	}
	if err := os.RemoveAll(path.Join(fc.wrkFolder, t.ppOrder.ID)); err != nil && !os.IsNotExist(err) {
		t.err = ErrFileSystem{err}
	}
	if t.err != nil {
		//filesystem err
		//log to cycle && close (state LoadWaie in cycle)
		//fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки:"+t.err.Error())
		_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrFileSystem, t.err.Error())
		return fc.closeTransform
	}

	req, err := grab.NewRequest(fl, t.ppOrder.DownloadLink)
	if err != nil {
		t.err = ErrService{err}
		//log to cycle && close (state LoadWaie in cycle)
		//fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
		_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrWeb, t.err.Error())
		return fc.closeTransform
	}
	req = req.WithContext(t.ctx)
	req.SkipExisting = true
	req.NoResume = true
	_ = fc.setCycleState(t, 0, pc.StateLoad, "start")
	t.loader = loader.Do(req)
	//waite till complete
	err = t.loader.Err()
	if err != nil {
		t.err = ErrService{err}
		//TODO err limiter
		//log to cycle && close (state LoadWaie in cycle)
		//fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
		_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrWeb, t.err.Error())
		return fc.closeTransform
	}
	//don't change cycle state just log
	_ = fc.setCycleState(t, 0, pc.StateLoadComplite, fmt.Sprintf("zip loaded elapsed=%s; speed=%.2f mb/s", t.loader.Duration().String(), t.loader.BytesPerSecond()/(1024*1024)))
	//still in LoadWaie in cycle
	//forvard to unzip
	return fc.unzip
}

//unpack zip
//in states: statePixelLoadStarted in PP and StateLoadWaite in cycle
//out states: statePixelLoadStarted in PP and StateUnzip (StateLoadWaite on error) in cycle
func (fc *Factory) unzip(t *Transform) stateFunc {
	//TODO err limiter??
	started := time.Now()
	fc.log("unzip", t.ppOrder.ID)
	_ = fc.setCycleState(t, 0, pc.StateUnzip, "start")
	reader, err := zip.OpenReader(filepath.Join(fc.wrkFolder, t.ppOrder.ID+".zip"))
	if err != nil {
		//broken zip?
		t.err = ErrService{err}
		//log to cycle && close (state LoadWaie in cycle)
		//fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
		_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrZip, t.err.Error())
		return fc.closeTransform
	}
	basePath := path.Join(fc.wrkFolder, t.ppOrder.ID)
	if err = os.MkdirAll(basePath, 0755); err != nil {
		t.err = ErrFileSystem{err}
		//log to cycle && close (state LoadWaie in cycle)
		//fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
		_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrFileSystem, t.err.Error())
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
			_ = os.MkdirAll(filePpath, file.Mode())
			continue
		}

		//create folder 4 file
		if err := os.MkdirAll(filepath.Dir(filePpath), 0755); err != nil {
			t.err = ErrFileSystem{err}
			//log to cycle && close (state LoadWaie in cycle)
			//fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
			_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrFileSystem, t.err.Error())
			return fc.closeTransform
		}

		//open in zip
		fileReader, err := file.Open()
		if err != nil {
			//broken zip?
			t.err = ErrService{err}
			//log to cycle && close (state LoadWaie in cycle)
			//fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
			_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrZip, t.err.Error())
			return fc.closeTransform
		}
		defer fileReader.Close()

		//create in filesystem
		targetFile, err := os.OpenFile(filePpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			t.err = ErrFileSystem{err}
			//log to cycle && close (state LoadWaie in cycle)
			//fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
			_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrFileSystem, t.err.Error())
			return fc.closeTransform
		}
		defer targetFile.Close()

		//extract
		if _, err := io.Copy(targetFile, fileReader); err != nil {
			//broken zip or file system err?
			t.err = ErrFileSystem{err}
			//log to cycle && close (state LoadWaie in cycle)
			//fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
			_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrZip, t.err.Error())
			return fc.closeTransform
		}
	}

	//move to StateUnzip in cycle (to resume from transform)
	_ = fc.setCycleState(t, pc.StateUnzip, pc.StateUnzip, fmt.Sprintf("complete elapsed=%s", time.Since(started).String()))

	//forvard to transform
	return fc.transformItems
}

//transformItems transforms orderitems to cycle orders
//in states: statePixelLoadStarted in PP and StateUnzip in cycle
//out states:
// if unziped folders or files not found (error ErrSourceNotFound) restart load
//		pp - statePixelStartLoad; cycle - StateLoadWaite
// if not all items processed move to soft stop, to proceed set statePixelConfirmed in pp (not implemented)
//		pp - statePixelWaiteConfirm; cycle - StatePreprocessIncomplite
// if error while creating orders in cycle (ErrRepository) or
// if error while set in PP StatePrinting
//	left as is, transform will restart late (not implemented)
//		pp - statePixelStartLoad; cycle - StateUnzip
// if error while move created orders to StatePreprocessWaite in cycle
// get by state StateLoadComplite check state StatePrinting in pp and try set cycle state again (not implemented)
//		pp - StatePrinting; cycle - base: StateUnzip; orders: StateLoadComplite
func (fc *Factory) transformItems(t *Transform) stateFunc {
	var err error
	started := time.Now()

	fc.log("transformItems", t.ppOrder.ID)
	_ = fc.setCycleState(t, 0, pc.StateTransform, "start")

	items, err := fc.ppClient.GetOrderItems(t.ctx, t.ppOrder.ID)
	if err != nil {
		//TODO restart?
		t.err = ErrService{err}
		//log to cycle && close (state StateUnzip in cycle)
		//fc.setPixelState(t, statePixelStartLoad, "Перезапуск загрузки из за ошибки: "+t.err.Error())
		_ = fc.setCycleState(t, pc.StateUnzip, pc.StateErrWeb, t.err.Error())
		return fc.closeTransform
	}

	//TODO check if cycle allready print suborder??
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
				msg := fmt.Sprintf("Перезапуск загрузки. Элемент заказа %s '%s'. Ошибка %s", co.SourceID, item.Name, err.Error())
				if _, ok := err.(ErrSourceNotFound); ok == true {
					//unziped folder deleted??
					//reset to reload & close transform
					_ = fc.setPixelState(t, statePixelStartLoad, msg)
					_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrPreprocess, msg)
					return fc.closeTransform
				}
				//some other err
				incomlete = true
				//just log error
				fc.log("error", msg)
				_ = fc.setPixelState(t, "", msg)
				_ = fc.setCycleState(t, 0, pc.StateErrPreprocess, msg)
			} else {
				co.ExtraInfo = buildExtraInfo(co, item)
				co.State = pc.StateLoadComplite
				orders = append(orders, co)
			}
		} else {
			//TODO some other
			//log error
			incomlete = true
			msg := fmt.Sprintf("Элемент заказа %s '%s' не обработан. Для продукта не настроены параметры подготовки", co.SourceID, item.Name)
			fc.log("error", msg)
			_ = fc.setPixelState(t, "", msg)
			_ = fc.setCycleState(t, 0, pc.StateErrPreprocess, msg)
		}
	}
	if incomlete {
		//log error ??
		msg := "Часть элементов заказа не обработано. Заказ не размещен в Photocycle"
		fc.log("error", msg)
		_ = fc.setPixelState(t, statePixelWaiteConfirm, msg)
		_ = fc.setCycleState(t, pc.StatePreprocessIncomplite, pc.StateErrPreprocess, msg)
	} else {
		//clear sub orders
		err = fc.pcClient.ClearGroup(t.ctx, t.pcBaseOrder.GroupID, t.pcBaseOrder.ID)
		if err != nil {
			t.err = ErrRepository{err}
			//fc.setPixelState(t, "", fmt.Sprintf("Ошибка БД: %s", err.Error()))
			_ = fc.setCycleState(t, pc.StateUnzip, pc.StateErrWrite, err.Error())
			return fc.closeTransform
		}
		//create sub orders in  pc.StateConfirmation
		for _, co := range orders {
			err = fc.pcClient.CreateOrder(t.ctx, co)
			if err != nil {
				t.err = ErrRepository{err}
				//fc.setPixelState(t, "", fmt.Sprintf("Ошибка БД: %s", err.Error()))
				_ = fc.setCycleState(t, pc.StateUnzip, pc.StateErrWrite, err.Error())
				return fc.closeTransform
			}
			_ = fc.pcClient.AddExtraInfo(t.ctx, co.ExtraInfo)
		}

		//finalase
		//TODO check production
		//set state in pp
		err = fc.ppClient.SetOrderStatus(t.ctx, t.ppOrder.ID, pp.StatePrinting, true)
		if err != nil {
			//TODO report err??
			t.err = ErrService{err}
			//fc.setPixelState(t, statePixelWaiteConfirm, fmt.Sprintf("Ошибка БД: %s", err.Error()))
			_ = fc.setCycleState(t, pc.StateUnzip, pc.StateErrWeb, fmt.Sprintf("Ошибка смены статуса на сайте. Статус:%s; Ошибка:%s ", pp.StatePrinting, err.Error()))
			return fc.closeTransform
		}
		//move all to waite preprocess to start in cycle
		err = fc.pcClient.SetGroupState(t.ctx, pc.StatePreprocessWaite, t.pcBaseOrder.GroupID, t.pcBaseOrder.ID)
		if err != nil {
			//TODO report err??
			t.err = ErrRepository{err}
			//fc.setPixelState(t, "", fmt.Sprintf("Ошибка БД: %s", err.Error()))
			_ = fc.setCycleState(t, pc.StateUnzip, pc.StateErrWrite, err.Error())
			return fc.closeTransform
		}

		//move main to complite state
		_ = fc.setCycleState(t, pc.StateSkiped, pc.StateTransform, fmt.Sprintf("complete elapsed=%s", time.Since(started).String()))

		//TODO finalize
		//kill zip and unziped folder
		_ = os.Remove(filepath.Join(fc.wrkFolder, t.ppOrder.ID+".zip"))
		_ = os.RemoveAll(path.Join(fc.wrkFolder, t.ppOrder.ID))
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
		_ = fc.ppClient.AddOrderComment(t.ctx, t.ppOrder.ID, fc.ppUser, message)
	}
	return err
}

func (fc *Factory) setCycleState(t *Transform, state int, logState int, message string) error {
	var err error
	if state != 0 {
		err = fc.pcClient.SetOrderState(t.ctx, t.pcBaseOrder.ID, state)
	}
	if message != "" || logState != 0 {
		_ = fc.pcClient.LogState(t.ctx, t.pcBaseOrder.ID, logState, message)
	}
	return err
}
