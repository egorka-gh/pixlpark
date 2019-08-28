package transform

import (
	"context"
	"sync"
	"time"
)

// Manager is queue manager of transform items (Transform)
type Manager struct {
	factory     *Factory
	concurrency int
	interval    int //sleep interval in min

	stop       chan struct{} //stop signal on close
	stoped     chan struct{} //manager stoped on close or nil
	cancel     context.CancelFunc
	mu         sync.Mutex            // guards transforms
	transforms map[string]*Transform // ID -> transform
}

//provider creates and run trusforms (factory function)
type provider func(ctx context.Context) *Transform

//Stop stops manager
//blocks while manager stops
func (m *Manager) Stop() {
	if m.IsRunning() == false {
		return
	}
	//cancel current context
	if m.cancel != nil {
		m.cancel()
	}
	//signal stop
	if m.stop != nil {
		close(m.stop)
		m.stop = nil
	}
	//waite till manager stops
	<-m.stoped
}

//IsRunning is manager started
func (m *Manager) IsRunning() bool {
	if m.stoped == nil {
		return false
	}
	select {
	case <-m.stoped:
		return false
	default:
		return true
	}

}

//Start starts manager, blocks caler
//if allready started returns immediately
//when Stop called calcels/stops current operation and then exits
func (m *Manager) Start() {
	//lock waile check/init service chanels
	m.mu.Lock()
	//check if started
	if m.IsRunning() {
		m.mu.Unlock()
		return
	}
	m.stoped = make(chan struct{}, 0)
	m.stop = make(chan struct{}, 0)
	m.mu.Unlock()

	defer func() {
		if m.cancel != nil {
			m.cancel()
		}
	}()

	//init
	if m.concurrency < 1 {
		m.concurrency = 1
	}
	if m.interval < 5 {
		m.interval = 5
	}

	var err error
	var ctx context.Context

	//TODO need it? if sorefactor to func
	//run revers sequense, restart stuck orders	first then new
	ctx, m.cancel = context.WithCancel(context.Background())
	//TODO add restarters
	//run regular fetching
	if err == nil && ctx.Err() == nil {
		err = m.runQueue(ctx, m.factory.Do, true)
	}

	/*
		//check if canceled
		if ctx.Err() != nil {
			return
		}
	*/

	//release context
	m.cancel()

	//run regular fetching vs specified interval
	alive := true
	for alive {
		//sleep
		tm := time.NewTimer(time.Duration(m.interval*60) * time.Second)
		select {
		case <-m.stop:
			//stop while sleep
			alive = false
		case <-tm.C:
			//recreate context
			ctx, m.cancel = context.WithCancel(context.Background())
			m.doWork(ctx)
			//release context
			m.cancel()
		}
	}
	//stoped
	close(m.stoped)
}

//run regular sequense, new first then restart stuck orders
func (m *Manager) doWork(ctx context.Context) {
	err := m.runQueue(ctx, m.factory.Do, true)
	if err != nil || ctx.Err() != nil {
		return
	}
	//TODO add restarters
}

//TODO add transforms limit??
func (m *Manager) runQueue(ctx context.Context, provider provider, monitor bool) (err error) {
	sem := make(chan bool, m.concurrency)
	run := true
	for run {
		sem <- true
		//fetch next transform
		//TODO check context done?
		t := provider(ctx)
		if t.IsComplete() {
			if _, ok := t.Err().(ErrEmptyQueue); ok == false {
				err = t.Err()
			}
			//release semafor
			<-sem
			//stop loop
			run = false
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
