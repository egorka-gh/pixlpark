package transform

import (
	"context"
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

	//current transforms
	mu         sync.Mutex            // guards transforms
	transforms map[string]*Transform // ID -> transform

	debug bool
}

//provider creates and run trusforms (factory function)
type provider func(ctx context.Context) *Transform

//IsStarted is manager started
func (m *Manager) IsStarted() bool {
	return m.chControl != nil
}

//IsRunning is manager running or paused
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
			//run regular fetching vs specified interval
			//create context
			ctx, m.cancel = context.WithCancel(context.Background())
			m.doWork(ctx)
			//release context
			m.cancel()
			//sleep
			m.chWork = nil
			m.timer = time.AfterFunc(time.Duration(m.interval)*time.Second, m.play)
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
		m.play()
		return
	}

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
	if m.timer != nil {
		m.timer.Stop()
	}
	if m.cancel != nil {
		m.cancel()
	}
	m.chWork = nil
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
	if m.timer != nil {
		m.timer.Stop()
	}
	m.chWork = m.chWorkBackup
	m.chControl <- struct{}{}
}

//Quit cancels current operation and stops manager machine
//non blocking
//any calls to Start and Pause will panic (send to closed chControl)
func (m *Manager) Quit() {
	if m.timer != nil {
		m.timer.Stop()
	}
	if m.cancel != nil {
		m.cancel()
	}
	m.chWork = nil
	close(m.chControl)
}

//Wait blocks caller till manager stops
func (m *Manager) Wait() {
	m.wg.Wait()
}

//run regular sequense, new first then restart stuck orders
func (m *Manager) doWork(ctx context.Context) {
	//load new
	err := m.runQueue(ctx, m.factory.LoadNew, true)
	if err != nil || ctx.Err() != nil {
		return
	}

	//run restarters

	//finalize prepared
	err = m.runQueue(ctx, m.factory.FinalizeRestart, true)
	if err != nil || ctx.Err() != nil {
		return
	}
	//restart broken transforms
	err = m.runQueue(ctx, m.factory.TransformRestart, true)
	if err != nil || ctx.Err() != nil {
		return
	}
	//restart broken loads
	err = m.runQueue(ctx, m.factory.LoadRestart, true)
	if err != nil || ctx.Err() != nil {
		return
	}

}

//TODO add transforms limit??
func (m *Manager) runQueue(ctx context.Context, provider provider, monitor bool) (err error) {
	sem := make(chan bool, m.concurrency)
	for {
		//waite concurrency
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
