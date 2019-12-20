package transform

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"

	log "github.com/go-kit/kit/log"
)

//NewManager creates manager
func NewManager(factory Factory, concurrency, interval int, logger log.Logger) *Manager {
	if concurrency < 1 {
		concurrency = 1
	}
	if interval < 5 {
		interval = 5
	}
	interval = interval * 60
	if logger == nil {
		logger = log.NewNopLogger()
	}

	return &Manager{
		factory:     factory,
		concurrency: concurrency,
		interval:    interval,
		logger:      logger,
	}
}

// Manager is queue manager of transform items (Transform)
type Manager struct {
	factory     Factory
	concurrency int
	interval    int //sleep interval in sec
	logger      log.Logger

	//run control
	chWork       <-chan struct{}
	chWorkBackup <-chan struct{}
	chControl    chan struct{}
	timer        *time.Timer

	wg     sync.WaitGroup
	cancel context.CancelFunc

	//last time run for daily tasks
	dailyTasksTime time.Time

	//current transforms
	mu         sync.Mutex            // guards transforms
	transforms map[string]*Transform // ID -> transform

	//current state (human-friendly)
	currState string
	isPaused  bool
	debug     bool
}

//provider creates and run trusforms (factory function)
type provider func(ctx context.Context) *Transform

//IsStarted is manager started
func (m *Manager) IsStarted() bool {
	return m.chControl != nil
}

//IsRunning is manager running, false if paused or sleeping
func (m *Manager) IsRunning() bool {
	return m.chWork != nil
}

//machine main routine
//periodicaly fetch and transfom pending orders
//can be paused and resumed
func (m *Manager) machine() {
	//clean up
	defer func() {
		m.wg.Done()
	}()

	var (
		//err error
		ctx context.Context
	)
Loop:
	for {
		select {
		case <-m.chWork:
			//run work vs specified interval
			//create context
			ctx, m.cancel = context.WithCancel(context.Background())
			m.doWork(ctx)
			//contex canceled?
			if ctx.Err() == nil {
				//release context
				m.cancel()
				//sleep
				m.mu.Lock()
				m.chWork = nil
				m.timer = time.AfterFunc(time.Duration(m.interval)*time.Second, m.play)
				m.mu.Unlock()
			}
		case _, ok := <-m.chControl:
			//flow control
			if ok {
				//restart loop
				continue Loop
			}
			// quit for
			break Loop
		}
	}
	//TODO finalizers??
}

//machine control

//Start starts manager machine, don't blocks caller
//if allready started just unblock machine
func (m *Manager) Start() {
	//resume if started
	if m.IsStarted() {
		m.isPaused = false
		m.play()
		return
	}
	m.logger.Log("Start", "")

	//init control
	// chWork, chWorkBackup
	ch := make(chan struct{})
	close(ch)
	m.chWork = ch
	m.chWorkBackup = ch
	// chControl
	m.chControl = make(chan struct{})

	// wg
	m.wg = sync.WaitGroup{}
	m.wg.Add(1)

	go m.machine()
}

//Pause cancels current operation and blocks manager machine
//non blocking
func (m *Manager) Pause() {
	defer func() {
		//recover from posible write to closed chan
		if r := recover(); r != nil {
			m.logger.Log("Pause.Panic", r)
		}
	}()
	if !m.IsStarted() {
		return
	}
	if m.cancel != nil {
		m.cancel()
	}

	m.mu.Lock()
	if m.timer != nil {
		m.timer.Stop()
	}
	m.chWork = nil
	m.isPaused = true
	m.currState = "Пауза"
	m.mu.Unlock()

	m.chControl <- struct{}{}
}

//play resume machine
//non blocking
func (m *Manager) play() {
	defer func() {
		//recover from posible write to closed chan
		if r := recover(); r != nil {
			m.logger.Log("play.Panic", r)
		}
	}()
	if !m.IsStarted() {
		return
	}
	if m.IsRunning() {
		return
	}
	m.mu.Lock()
	if m.timer != nil {
		m.timer.Stop()
	}
	m.chWork = m.chWorkBackup
	m.mu.Unlock()
	m.chControl <- struct{}{}
}

//Quit cancels current operation and stops manager machine
//non blocking
//any calls to Start and Pause will panic (send to closed chControl)
func (m *Manager) Quit() {
	m.logger.Log("Quit", "")
	if m.timer != nil {
		m.timer.Stop()
	}
	if m.cancel != nil {
		m.cancel()
	}
	m.chWork = nil
	m.currState = "Остановка"
	close(m.chControl)
}

//Wait blocks caller till manager stops
func (m *Manager) Wait() {
	m.wg.Wait()
}

//ManagerInfo holds manager info
type ManagerInfo struct {
	StateCaption  string  `json:"caption"`
	IsRunning     bool    `json:"running"`
	IsPaused      bool    `json:"paused"`
	Threads       int     `json:"threads"`
	OrderIds      string  `json:"ids"`
	OrderCount    int     `json:"count"`
	QueueLen      int     `json:"queue"`
	DownloadSpeed float64 `json:"speed"`
}

//GetInfo returns ManagerInfo
func (m *Manager) GetInfo() (info ManagerInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var speed float64
	ids := make([]string, 0, m.concurrency)
	for key := range m.transforms {
		ids = append(ids, key)
		speed += m.transforms[key].BytesPerSecond()
	}
	inf := ManagerInfo{
		StateCaption:  m.currState,
		IsRunning:     m.IsRunning(),
		IsPaused:      m.isPaused,
		Threads:       m.concurrency,
		OrderIds:      strings.Join(ids[:], ","),
		OrderCount:    len(ids),
		QueueLen:      m.factory.QueueLen(),
		DownloadSpeed: math.Round(speed*100/float64(1024*1024)) / 100,
	}

	return inf
}

//run regular sequense, new first then restart stuck orders
func (m *Manager) doWork(ctx context.Context) {
	//load new
	m.currState = "Загрузка"
	err := m.runQueue(ctx, m.factory.LoadNew, true)
	if err != nil || ctx.Err() != nil {
		m.logNotNilErr("LoadNew", err, ctx.Err())
		return
	}

	//run restarters

	//finalize prepared
	m.currState = "Перезапуск не завершенных"
	err = m.runQueue(ctx, m.factory.FinalizeRestart, false)
	if err != nil || ctx.Err() != nil {
		m.logNotNilErr("FinalizeRestart", err, ctx.Err())
		return
	}
	//restart broken transforms
	m.currState = "Перезапуск не подготовленных"
	err = m.runQueue(ctx, m.factory.TransformRestart, false)
	if err != nil || ctx.Err() != nil {
		m.logNotNilErr("TransformRestart", err, ctx.Err())
		return
	}
	//restart broken loads
	m.currState = "Перезапуск не загруженных"
	err = m.runQueue(ctx, m.factory.LoadRestart, true)
	if err != nil || ctx.Err() != nil {
		m.logNotNilErr("LoadRestart", err, ctx.Err())
		return
	}

	//daily tasks
	if m.dailyTasksTime.IsZero() || time.Since(m.dailyTasksTime).Hours() > 24 {
		m.dailyTasksTime = time.Now()
		//sync cycle vs pixel
		m.currState = "Синхронизация статусов"
		err = m.runQueue(ctx, m.factory.SyncCycle, false)
		if err != nil || ctx.Err() != nil {
			m.logNotNilErr("SyncCycle", err, ctx.Err())
			return
		}
	}

	m.currState = "Ожидание"
}

func (m *Manager) logNotNilErr(key string, errs ...error) {
	for _, e := range errs {
		if e != nil {
			m.logger.Log(key, e)
			break
		}
	}
}

//TODO add transforms limit??
func (m *Manager) runQueue(ctx context.Context, provider provider, monitor bool) (err error) {
	sem := make(chan bool, m.concurrency)
	for {
		//waite concurrency limit
		sem <- true
		//check context done
		if ctx.Err() != nil {
			//canceled
			err = ctx.Err()
			//release semafor
			<-sem
			//stop loop
			break
		}
		//fetch next transform
		t := provider(ctx)
		if t.IsComplete() {
			if _, ok := t.Err().(ErrEmptyQueue); ok == false {
				err = t.Err()
			}
			//release semafor
			<-sem
			//stop loop
			break
		} else {
			//monitor transform
			if monitor {
				m.monTransform(t.ID(), t)
			}
			//waite till transform complite
			go func(t *Transform) {
				//release semafor
				defer func() { <-sem }()
				//block till complite
				t.Wait()
				//remove from monitor
				m.monTransform(t.ID(), nil)
			}(t)
		}
	}
	//waite started transforms
	for i := 0; i < cap(sem); i++ {
		sem <- true
	}
	close(sem)
	return
}

func (m *Manager) monTransform(id string, transform *Transform) {
	if id == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.transforms == nil {
		m.transforms = make(map[string]*Transform)
	}
	if transform == nil {
		delete(m.transforms, id)
	} else {
		m.transforms[id] = transform
	}
}
