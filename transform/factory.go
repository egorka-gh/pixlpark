package transform

import (
	"archive/zip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	log "github.com/go-kit/kit/log"

	"github.com/cavaliercoder/grab"
	pc "github.com/egorka-gh/pixlpark/photocycle"
	pp "github.com/egorka-gh/pixlpark/pixlpark/service"
)

// Factory is factory of transform item (Transform)
// creates transform item and defines transform process
type Factory interface {
	// LoadNew main sequence
	// fetch new order and perfom full trunsform
	LoadNew(ctx context.Context) *Transform
	//SoftErrorRestart restarts orders after confirmed soft error
	//SoftErrorRestart(ctx context.Context) *Transform
	//LoadRestart reload orders that allready started (statePixelLoadStarted)
	LoadRestart(ctx context.Context) *Transform
	//TransformRestart restart incompleted transforms
	TransformRestart(ctx context.Context) *Transform
	//FinalizeRestart updates states for complited transforms
	FinalizeRestart(ctx context.Context) *Transform

	// DoOrder loads order and perfom full trunsform (4 tests only)
	DoOrder(ctx context.Context, id string) *Transform

	//QueueLen returns current queues lenth
	QueueLen() int

	SetDebug(debug bool)
}

// Factory is factory of transform item (Transform)
// creates transform item and defines transform process
type baseFactory struct {
	production     int
	ppClient       pp.PPService
	pcClient       pc.Repository
	source         int
	wrkFolder      string
	cycleFolder    string
	cyclePrtFolder string
	ppUser         string
	logger         log.Logger
	Debug          bool

	//current queues
	mu     sync.Mutex            // guards queues map
	queues map[string][]pp.Order // PP state -> orders slice

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
	errCantTransform = ErrCantTransform{errors.New("Для продукта не настроены параметры подготовки")}
)

// NewFactory returns a new transform Factory, using provided configuration.
//TODO refactor to config
func NewFactory(pixlparkClient pp.PPService, photocycleClient pc.Repository, sourse, production int, workFolder, cycleFolder, cyclePrtFolder, pixlparkUserEmail string, logger log.Logger) Factory {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	return &baseFactory{
		production:     production,
		ppClient:       pixlparkClient,
		pcClient:       photocycleClient,
		source:         sourse,
		wrkFolder:      workFolder,
		cycleFolder:    cycleFolder,
		cyclePrtFolder: cyclePrtFolder,
		ppUser:         pixlparkUserEmail,
		logger:         logger,
		queues:         make(map[string][]pp.Order),
	}
}

func (fc *baseFactory) SetDebug(debug bool) {
	fc.Debug = debug
}

// LoadNew main sequence
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
func (fc *baseFactory) LoadNew(ctx context.Context) *Transform {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	t := &Transform{
		fetchState: statePixelStartLoad,
		Start:      time.Now(),
		Done:       make(chan struct{}, 0),
		ctx:        ctx,
		cancel:     cancel,
		logger:     log.With(fc.logger, "sequence", "LoadNew"),
	}

	t.logger.Log("event", "start")

	// Run state-machine while caller is blocked to fetch pixelpark order and to initialize transform.
	fc.run(t, fc.fetchToLoad)
	if t.IsComplete() {
		if t.Err() != nil {
			t.logger.Log("event", "end", "error", t.Err().Error())
		} else {
			//panic??
			t.logger.Log("event", "end", "error", "complited fetch must return error")
		}
		return t
	}
	//Run load in a new goroutine
	go fc.run(t, fc.loadZIP)
	return t
}

//LoadRestart reload orders that allready started (statePixelLoadStarted)
// but not complete for some reason (service stop, some error while load, unzip)
// also restarts orders after soft error statePixelWaiteConfirm, incomplited transform confirmed by user
// behavior same as LoadNew exept fetch orders in state statePixelLoadStarted
// get ordrers vs statePixelLoadStarted in PP && StateLoadWaite in Cycle
func (fc *baseFactory) LoadRestart(ctx context.Context) *Transform {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	t := &Transform{
		fetchState: statePixelLoadStarted,
		Start:      time.Now(),
		Done:       make(chan struct{}, 0),
		ctx:        ctx,
		cancel:     cancel,
		logger:     log.With(fc.logger, "sequence", "LoadRestart"),
	}

	// Run state-machine while caller is blocked to fetch pixelpark order and to initialize transform.
	t.logger.Log("event", "start")
	fc.run(t, fc.fetchToLoad)
	if t.IsComplete() {
		if t.Err() != nil {
			t.logger.Log("event", "end", "error", t.Err().Error())
		} else {
			//panic??
			t.logger.Log("event", "end", "error", "complited fetch must return error")
		}
		return t
	}
	//Run load in a new goroutine
	go fc.run(t, fc.loadZIP)
	return t
}

/*
//SoftErrorRestart reload orders in state statePixelConfirmed
// orders that allready started and has some reparable error (now orders that not transformed - unknown product)
// behavior same as LoadNew exepct fetch orders in state statePixelConfirmed
// resumes processing from transformItems
// get ordrers vs statePixelConfirmed in PP, expected state in cycle StateUnzip (expect but not check)
//TODO buggi state seq
//либо рестартовать чз statePixelLoadStarted, либо переставить statePixelConfirmed и statePixelConfirmed
func (fc *baseFactory) SoftErrorRestart(ctx context.Context) *Transform {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	t := &Transform{
		fetchState: statePixelConfirmed,
		Start:      time.Now(),
		Done:       make(chan struct{}, 0),
		ctx:        ctx,
		cancel:     cancel,
		logger:     log.With(fc.logger, "sequence", "SoftErrorRestart"),
	}

	// Run state-machine while caller is blocked to fetch pixelpark order and to initialize transform.
	t.logger.Log("event", "start")
	fc.run(t, fc.fetchToLoad)
	if t.IsComplete() {
		if t.Err() != nil {
			t.logger.Log("event", "end", "error", t.Err().Error())
		} else {
			//panic??
			t.logger.Log("event", "end", "error", "complited fetch must return error")
		}
		return t
	}

	//Run transform in a new goroutine
	go fc.run(t, fc.transformItems)

	return t
}
*/

//TransformRestart restart incompleted transforms
// orders that loaded and unziped but not complete for some reason (service stop or some error while transform)
// behavior same as LoadNew exepct fetch orders method
// get ordrers vs statePixelLoadStarted in PP && StateUnzip in Cycle
func (fc *baseFactory) TransformRestart(ctx context.Context) *Transform {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	t := &Transform{
		fetchState: statePixelLoadStarted,
		Start:      time.Now(),
		Done:       make(chan struct{}, 0),
		ctx:        ctx,
		cancel:     cancel,
		logger:     log.With(fc.logger, "sequence", "TransformRestart"),
	}

	t.logger.Log("event", "start")
	// Run state-machine while caller is blocked to fetch pixelpark order and to initialize transform.
	fc.run(t, fc.fetchToTransform)
	if t.IsComplete() {
		t.logger.Log("event", "end", "error", t.Err().Error())
		return t
	}
	//Run transform in a new goroutine
	go fc.run(t, fc.transformItems)
	return t
}

//FinalizeRestart updates states for complited transforms
// orders in cycle that hangs in StateLoadComplite on error when moved to StatePreprocessWaite (writeLock or some else)
// see transformItems SetGroupState at the end
// behavior is different
// allways returns complited transform
// do all in one step
func (fc *baseFactory) FinalizeRestart(ctx context.Context) *Transform {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	t := &Transform{
		fetchState: statePixelLoadStarted,
		Start:      time.Now(),
		Done:       make(chan struct{}, 0),
		ctx:        ctx,
		cancel:     cancel,
		logger:     log.With(fc.logger, "sequence", "FinalizeRestart"),
	}

	// get and forward all
	t.logger.Log("event", "start")
	fc.run(t, fc.doFinalize)
	if t.IsComplete() {
		//complited
		t.logger.Log("event", "end", "error", t.Err().Error())
		return t
	}

	//must never happend
	t.logger.Log("event", "end", "error", "Got incompleted transform")
	fc.run(t, fc.closeTransform)
	return t
}

//QueueLen returns current queues lenth
func (fc *baseFactory) QueueLen() int {
	var res int
	fc.mu.Lock()
	defer fc.mu.Unlock()
	for key := range fc.queues {
		res += len(fc.queues[key])
	}
	return res
}

// An stateFunc is factory unit of work
// is an action that mutates the state of a Transform and returns
// the next stateFunc to be called.
type stateFunc func(*Transform) stateFunc

// run calls the given stateFunc function and all subsequent returned stateFuncs
// until a stateFunc returns nil or the Transform.ctx is canceled. Each stateFunc
// should mutate the state of the given Transform until it has completed or failed.
func (fc *baseFactory) run(t *Transform, f stateFunc) {
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

//fetchToLoad looks for in PP the next order to process
//bloks till get some order to process
//on success t is not closed (valid for processing)
//if there is no orders returns ErrEmptyQueue
//fetch in PP state statePixelStartLoad or statePixelLoadStarted
//fetched order moves to statePixelLoadStarted in PP and to StateLoadWaite in cycle
//must check order state in cycle (not implemented)
func (fc *baseFactory) fetchToLoad(t *Transform) stateFunc {
	var err error
	var inerErr error
	var po pp.Order
	t.logger.Log("fetchState", t.fetchState)

	for po, err = fc.queuePop(t.ctx, t.fetchState); err == nil; {
		logger := log.With(t.logger, "order", po.ID)
		logger.Log("fetch", "start")
		t.ppOrder = po

		co := fromPPOrder(po, fc.source, "@")
		co.State = pc.StateLoadWaite

		//check production
		if fc.production > 0 && po.ProductionID != fc.production {
			logger.Log("skip", "production")
			//log if it created ??
			//fc.pcClient.LogState(t.ctx, co.ID, pc.StateErrProductionNotSet, "")
			continue
		}

		//check states in cycle
		ok := false
		ok, inerErr = fc.checkCreateInCycle(t, co)
		if !ok && inerErr != nil {
			logger.Log("skip", inerErr.Error())
			continue
		}
		//move state in PP to statePixelLoadStarted
		if t.fetchState != statePixelLoadStarted {
			if inerErr = fc.setPixelState(t, statePixelLoadStarted, ""); inerErr != nil {
				//keep fetching
				logger.Log("skip", inerErr.Error())
				inerErr = ErrService{inerErr}
				continue
			}
		}
		//сan load stop fetching
		t.pcBaseOrder = co
		//next t logs will go vs order id
		t.logger = logger
		logger.Log("fetch", "complite")
		return nil
	}

	t.err = err
	if err == nil {
		//return last error
		t.err = inerErr
	}

	return fc.closeTransform
}

func (fc *baseFactory) queuePop(ctx context.Context, state string) (pp.Order, error) {
	var order pp.Order
	var err error
	fc.mu.Lock()
	defer fc.mu.Unlock()
	//get queue slice
	queue := fc.queues[state]
	if queue == nil {
		//get orders count by state
		cnt := 0
		cnt, err = fc.ppClient.CountOrders(ctx, []string{state})
		if err != nil {
			return order, ErrService{err}
		}
		if cnt == 0 {
			return order, ErrEmptyQueue{fmt.Errorf("No orders in state %s", state)}
		}
		//get all orders
		queue, err = fc.ppClient.GetOrders(ctx, state, 0, 0, cnt, 0)
		if err != nil {
			return order, ErrService{err}
		}
	}
	//check if empty
	if len(queue) == 0 {
		fc.queues[state] = nil
		return order, ErrEmptyQueue{fmt.Errorf("No orders in state %s", state)}
	}
	//pop last (first by date, GetOrders returns orders sorted by date DESC)
	order = queue[len(queue)-1]
	fc.queues[state] = queue[:len(queue)-1]

	return order, err
}

//fetchToTransform looks for in cycle orders whith not complited transform
//bloks till get some order to process
//on success t is not closed (valid for processing)
//if there is no orders returns ErrEmptyQueue
//fetch in PP state statePixelLoadStarted & cycle state StateUnzip
func (fc *baseFactory) fetchToTransform(t *Transform) stateFunc {
	//get one from cycle
	var err error
	t.pcBaseOrder, err = fc.pcClient.LoadBaseOrderByState(t.ctx, fc.source, pc.StateUnzip)
	if err != nil {
		if err != sql.ErrNoRows {
			t.err = ErrRepository{err}
		} else {
			t.err = ErrEmptyQueue{fmt.Errorf("No orders in cycle with state StateUnzip")}
		}
		return fc.closeTransform
	}
	logger := log.With(t.logger, "order", t.pcBaseOrder.GroupID)
	logger.Log("event", "start")

	//load from PP
	t.ppOrder, err = fc.ppClient.GetOrder(t.ctx, t.pcBaseOrder.SourceID)
	if err != nil {
		t.err = ErrService{err}
		logger.Log("error", err.Error())
		return fc.closeTransform
	}
	//check PP state
	if t.ppOrder.Status != statePixelLoadStarted {
		//wrong state in PP
		//TODO reset in cycle ??
		msg := fmt.Sprintf("Wrong state '%s' in source site, expected '%s'", t.ppOrder.Status, statePixelLoadStarted)
		err = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrPreprocess, msg)
		if err != nil {
			t.err = ErrRepository{err}
			logger.Log("error", err.Error())
			return fc.closeTransform
		}
		// try next one
		return fc.fetchToTransform
	}

	//fetched
	t.logger = logger
	logger.Log("event", "end")
	return nil
}

//doFinalize loads all orders in state StateLoadComplite check in PP for StatePrinting
// and moves to state StatePreprocessWaite
//bloks till process all
//allways returns complited transform
func (fc *baseFactory) doFinalize(t *Transform) stateFunc {
	//get order list from DB
	orders, err := fc.pcClient.LoadBaseOrderByChildState(t.ctx, fc.source, pc.StateUnzip, pc.StateLoadComplite)
	if err != nil {
		if err != sql.ErrNoRows {
			t.err = ErrRepository{err}
		} else {
			t.err = ErrEmptyQueue{fmt.Errorf("No orders in cycle with appropriate states")}
		}
		return fc.closeTransform
	}
	//TODO continue on error?
	for _, t.pcBaseOrder = range orders {
		//check context canceled
		if t.ctx.Err() != nil {
			t.err = t.ctx.Err()
			return fc.closeTransform
		}
		logger := log.With(t.logger, "order", t.pcBaseOrder.GroupID)
		logger.Log("event", "start")

		//load from PP
		t.ppOrder, err = fc.ppClient.GetOrder(t.ctx, t.pcBaseOrder.SourceID)
		if err != nil {
			t.err = ErrService{err}
			logger.Log("error", err.Error())
			return fc.closeTransform
		}
		//check PP state
		if t.ppOrder.Status != pp.StatePrinting {
			//wrong state in PP
			//TODO reset in cycle ??
			msg := fmt.Sprintf("Wrong state '%s' in source site, expected '%s'", t.ppOrder.Status, pp.StatePrinting)
			err = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrPreprocess, msg)
			if err != nil {
				t.err = ErrRepository{err}
				logger.Log("error", err.Error())
				return fc.closeTransform
			}
		}
		//finalize
		err = fc.finish(t, true)
		if err != nil {
			logger.Log("error", err.Error())
			return fc.closeTransform
		}
		logger.Log("event", "end")
	}

	//complited
	t.err = ErrEmptyQueue{fmt.Errorf("No orders in cycle with appropriate states")}
	return fc.closeTransform
}

//start grab client to load zip
//in/out states statePixelLoadStarted in PP and StateLoadWaite in cycle
func (fc *baseFactory) loadZIP(t *Transform) stateFunc {
	logger := log.With(t.logger, "stage", "loadZIP")
	logger.Log("event", "start")

	loader := grab.NewClient()
	fl := filepath.Join(fc.wrkFolder, t.ppOrder.ID+".zip")
	//check delete old zip & folder
	if err := os.Remove(fl); err != nil && !os.IsNotExist(err) {
		t.err = ErrFileSystem{err}
	}
	if t.err == nil {
		if err := os.RemoveAll(path.Join(fc.wrkFolder, t.ppOrder.ID)); err != nil {
			t.err = ErrFileSystem{err}
		}
	}
	if t.err != nil {
		logger.Log("error", t.err.Error())
		//filesystem err
		_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrFileSystem, t.err.Error())
		return fc.closeTransform
	}

	req, err := grab.NewRequest(fl, t.ppOrder.DownloadLink)
	//TODO prevent reusing connection
	req.HTTPRequest.Close = true
	if err != nil {
		logger.Log("error", err.Error())
		t.err = ErrService{err}
		//log to cycle && close (state LoadWaie in cycle)
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
		logger.Log("error", err.Error())
		t.err = ErrService{err}
		//TODO err limiter
		//log to cycle && close (state LoadWaie in cycle)
		_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrWeb, t.err.Error())
		return fc.closeTransform
	}
	logger.Log("event", "end")
	//don't change cycle state just log
	_ = fc.setCycleState(t, 0, pc.StateLoadComplite, fmt.Sprintf("zip loaded elapsed=%s; speed=%.2f mb/s", t.loader.Duration().String(), t.loader.BytesPerSecond()/(1024*1024)))
	//still in LoadWaie in cycle
	//forvard to unzip
	return fc.unzip
}

//unpack zip
//in states: statePixelLoadStarted in PP and StateLoadWaite in cycle
//out states: statePixelLoadStarted in PP and StateUnzip (StateLoadWaite on error) in cycle
func (fc *baseFactory) unzip(t *Transform) stateFunc {
	//TODO err limiter??
	started := time.Now()
	logger := log.With(t.logger, "stage", "unzip")
	logger.Log("event", "start")
	_ = fc.setCycleState(t, 0, pc.StateUnzip, "start")

	reader, err := zip.OpenReader(filepath.Join(fc.wrkFolder, t.ppOrder.ID+".zip"))
	if err != nil {
		//broken zip?
		logger.Log("error", err.Error())
		t.err = ErrService{err}
		//log to cycle && close (state LoadWaie in cycle)
		_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrZip, t.err.Error())
		return fc.closeTransform
	}
	basePath := path.Join(fc.wrkFolder, t.ppOrder.ID)
	if err = os.MkdirAll(basePath, 0755); err != nil {
		logger.Log("error", err.Error())
		t.err = ErrFileSystem{err}
		//log to cycle && close (state LoadWaie in cycle)
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
			logger.Log("error", err.Error())
			t.err = ErrFileSystem{err}
			//log to cycle && close (state LoadWaie in cycle)
			_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrFileSystem, t.err.Error())
			return fc.closeTransform
		}

		//open in zip
		fileReader, err := file.Open()
		if err != nil {
			//broken zip?
			logger.Log("error", err.Error())
			t.err = ErrService{err}
			//log to cycle && close (state LoadWaie in cycle)
			_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrZip, t.err.Error())
			return fc.closeTransform
		}
		defer fileReader.Close()

		//create in filesystem
		targetFile, err := os.OpenFile(filePpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			logger.Log("error", err.Error())
			t.err = ErrFileSystem{err}
			//log to cycle && close (state LoadWaie in cycle)
			_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrFileSystem, t.err.Error())
			return fc.closeTransform
		}
		defer targetFile.Close()

		//extract
		if _, err := io.Copy(targetFile, fileReader); err != nil {
			//broken zip or file system err?
			logger.Log("error", err.Error())
			t.err = ErrFileSystem{err}
			//log to cycle && close (state LoadWaie in cycle)
			_ = fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrZip, t.err.Error())
			return fc.closeTransform
		}
	}

	//move to StateUnzip in cycle (to resume from transform)
	_ = fc.setCycleState(t, pc.StateUnzip, pc.StateUnzip, fmt.Sprintf("complete elapsed=%s", time.Since(started).String()))
	logger.Log("event", "end")
	//forvard to transform
	return fc.transformItems
}

//transformItems transforms orderitems to cycle orders
//in states: statePixelLoadStarted in PP and StateUnzip in cycle
//out states:
// if unziped folders or files not found (error ErrSourceNotFound) restart load
//		pp - statePixelStartLoad; cycle - StateLoadWaite
// if empty order (no items) hard error lock for processing
//		pp - statePixelAbort; cycle - StateSkiped
// if not all items processed move to soft stop, to proceed set statePixelConfirmed in pp (not implemented)
//		pp - statePixelWaiteConfirm; cycle - StatePreprocessIncomplite
// if error while creating orders in cycle (ErrRepository) or
// if error while set in PP StatePrinting
//	left as is, transform will restart late (not implemented)
//		pp - statePixelLoadStarted; cycle - StateUnzip
// if error while move created orders to StatePreprocessWaite in cycle
// get by state StateLoadComplite check state StatePrinting in pp and try set cycle state again (not implemented)
//		pp - StatePrinting; cycle - base: StateUnzip; orders: StateLoadComplite
func (fc *baseFactory) transformItems(t *Transform) stateFunc {
	var err error
	logger := log.With(t.logger, "stage", "transform")
	logger.Log("event", "start")
	_ = fc.setCycleState(t, 0, pc.StateTransform, "start")

	items, err := fc.ppClient.GetOrderItems(t.ctx, t.ppOrder.ID)
	if err != nil {
		//TODO restart?
		t.err = ErrService{err}
		//log to cycle && close (state StateUnzip in cycle)
		_ = fc.setCycleState(t, pc.StateUnzip, pc.StateErrWeb, t.err.Error())
		return fc.closeTransform
	}

	if len(items) == 0 {
		t.err = ErrTransform{errors.New("Order has no items")}
		logger.Log("error", t.err.Error())
		fc.setPixelState(t, statePixelAbort, "Пустой заказ")
		fc.setCycleState(t, pc.StateSkiped, pc.StateErrWeb, "Пустой заказ")
		if fc.Debug {
			fmt.Printf("Пустой заказ\n")
		}
		return fc.closeTransform
	}

	//TODO check if cycle allready print suborder??
	//process items
	orders := make([]pc.Order, 0, len(items))
	incomlete := false
	for i, item := range items {
		//process item
		l := log.With(logger, "item", item.ID)
		l.Log("transform", "start")
		//create cycle order
		co := fromPPOrder(t.ppOrder, fc.source, fmt.Sprintf("-%d", i))
		co.SourceID = fmt.Sprintf("%s-%d", t.ppOrder.ID, item.ID)

		isPhoto := false
		//try build by alias
		//intermediate state for buld by alias (then forward to StatePreprocessWaite)
		co.State = pc.StateLoadComplite
		err = fc.transformAlias(t.ctx, item, &co)
		if _, ok := err.(ErrCantTransform); ok == true {
			//try build photo print
			//intermediate state for buld photo (then forward to StatePrintWaite)
			isPhoto = true
			co.State = pc.StatePreprocessComplite
			err = fc.transformPhoto(t.ctx, item, &co)
		}
		if err != nil {
			incomlete = true
			msg := ""
			if _, ok := err.(ErrSourceNotFound); ok == true {
				//unziped folder deleted??
				//reset to reload & close transform
				msg = fmt.Sprintf("Перезапуск загрузки. Элемент заказа %s '%s'. Ошибка %s", co.SourceID, item.Name, err.Error())
				if fc.Debug {
					fmt.Printf("Элемент заказа %s '%s'. Ошибка %s\n", co.SourceID, item.Name, err.Error())
				}
				fc.setPixelState(t, statePixelStartLoad, msg)
				fc.setCycleState(t, pc.StateLoadWaite, pc.StateErrPreprocess, msg)
				return fc.closeTransform
			}
			msg = fmt.Sprintf("Элемент заказа %s '%s' не обработан. Ошибка %s", co.SourceID, item.Name, err.Error())
			if fc.Debug {
				fmt.Printf(msg)
			}
			//just log error
			l.Log("error", msg)
			fc.setPixelState(t, "", msg)
			fc.setCycleState(t, 0, pc.StateErrPreprocess, msg)
		} else {
			//item processed, add order
			exi := buildExtraInfo(co, item)
			if isPhoto {
				exi.Sheets = item.Quantity
				exi.Books = 1
			}
			co.ExtraInfo = exi
			orders = append(orders, co)
			l.Log("transform", "complite")
		}
	}

	if incomlete {
		//TODO restarter not implemented
		msg := "Часть элементов заказа не обработано. Заказ не размещен в Photocycle"
		logger.Log("error", msg)
		fc.setPixelState(t, statePixelWaiteConfirm, msg)
		fc.setCycleState(t, pc.StatePreprocessIncomplite, pc.StateErrPreprocess, msg)
		t.err = ErrTransform{errors.New(msg)}
	} else {
		//clear sub orders
		err = fc.pcClient.ClearGroup(t.ctx, fc.source, t.pcBaseOrder.GroupID, t.pcBaseOrder.ID)
		if err != nil {
			t.err = ErrRepository{err}
			_ = fc.setCycleState(t, pc.StateUnzip, pc.StateErrWrite, err.Error())
			logger.Log("error", err.Error())
			return fc.closeTransform
		}
		//create sub orders
		err = fc.pcClient.FillOrders(t.ctx, orders)
		if err != nil {
			t.err = ErrRepository{err}
			_ = fc.setCycleState(t, pc.StateUnzip, pc.StateErrWrite, err.Error())
			logger.Log("error", err.Error())
			return fc.closeTransform
		}
		logger.Log("event", "end")
		//finalase
		fc.finish(t, false)
	}

	//stop
	return fc.closeTransform
}

// closeTransform finalizes the Transform
func (fc *baseFactory) closeTransform(t *Transform) stateFunc {
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

//check states in cycle
func (fc *baseFactory) checkCreateInCycle(t *Transform, co pc.Order) (ok bool, err error) {
	//load/check state from all orders by group
	states, err := fc.pcClient.GetGroupState(t.ctx, co.ID, fc.source, co.GroupID)
	if err != nil {
		//TODO database not responding??
		err = ErrRepository{err}
		//stop all?
		return false, err
	}
	if states.BaseState == 0 && states.ChildState == 0 {
		//normal flow - create base
		if err = fc.pcClient.CreateOrder(t.ctx, co); err != nil {
			err = ErrRepository{err}
			return false, err
		}
		return true, nil
	}

	if states.BaseState == pc.StateCanceledPHCycle {
		//cancel in pp
		err = errors.New("Заказ отменен в PhotoCycle")
		_ = fc.setPixelState(t, statePixelAbort, err.Error())
		return false, err
	}
	if states.ChildState > pc.StatePrintWaite {
		//cancel in pp
		err = errors.New("Запуск заказа отправленного на печать в PhotoCycle")
		_ = fc.setPixelState(t, statePixelAbort, err.Error())
		return false, err
	}

	//check by fetch state
	//statePixelLoadStarted
	if t.fetchState == statePixelLoadStarted {
		//reload sequence
		if states.BaseState == pc.StateUnzip {
			//order for retransfom sequence (loaded but not transformed)
			//skip
			err = fmt.Errorf("Не соответствие статусов pixel:%s, cycle:StateUnzip", t.fetchState)
			//_ = fc.setPixelState(t, "", err.Error())
			return false, err
		}
	}
	/*
		//statePixelConfirmed
		if t.fetchState == statePixelConfirmed {
			//expects that zip loaded
			//if not or deleted - will restart from very beginning
			co.State = pc.StateUnzip
		}
	*/
	//clear sub orders
	if states.ChildState > 0 {
		_ = fc.pcClient.ClearGroup(t.ctx, fc.source, co.GroupID, co.ID)
	}
	//reset base state
	if states.BaseState != co.State {
		_ = fc.pcClient.SetOrderState(t.ctx, co.ID, co.State)
	}
	return true, nil
}

//finalize complete transform
func (fc *baseFactory) finish(t *Transform, restarted bool) (err error) {
	defer func() {
		if err != nil {
			t.logger.Log("stage", "finish", "error", err.Error())
		}
	}()

	if restarted == false && fc.Debug == false {
		//set state in pp
		err = fc.ppClient.SetOrderStatus(t.ctx, t.ppOrder.ID, pp.StatePrinting, true)
		if err != nil {
			t.err = ErrService{err}
			fc.setCycleState(t, pc.StateUnzip, pc.StateErrWeb, fmt.Sprintf("Ошибка смены статуса на сайте. Статус:%s; Ошибка:%s ", pp.StatePrinting, err.Error()))
			return err
		}
	}

	//start orders in cycle
	err = fc.pcClient.StartOrders(t.ctx, fc.source, t.pcBaseOrder.GroupID, t.pcBaseOrder.ID)
	if err != nil {
		t.err = ErrRepository{err}
		fc.setCycleState(t, pc.StateUnzip, pc.StateErrWrite, err.Error())
		return err
	}

	//move main to complite state
	fc.setCycleState(t, pc.StateSkiped, pc.StateTransform, "complete")

	//kill zip and unziped folder
	if !fc.Debug {
		os.Remove(filepath.Join(fc.wrkFolder, t.ppOrder.ID+".zip"))
		os.RemoveAll(path.Join(fc.wrkFolder, t.ppOrder.ID))
	}
	return nil
}

func (fc *baseFactory) setPixelState(t *Transform, state, message string) error {
	var err error
	if fc.Debug {
		//do nothing
		return nil
	}
	if state != "" {
		err = fc.ppClient.SetOrderStatus(t.ctx, t.ppOrder.ID, state, false)
	}
	if message != "" {
		_ = fc.ppClient.AddOrderComment(t.ctx, t.ppOrder.ID, fc.ppUser, message)
	}
	return err
}

func (fc *baseFactory) setCycleState(t *Transform, state int, logState int, message string) error {
	var err error
	if state != 0 {
		err = fc.pcClient.SetOrderState(t.ctx, t.pcBaseOrder.ID, state)
	}
	if message != "" || logState != 0 {
		_ = fc.pcClient.LogState(t.ctx, t.pcBaseOrder.ID, logState, message)
	}
	return err
}
