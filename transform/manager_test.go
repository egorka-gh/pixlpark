package transform

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	pp "github.com/egorka-gh/pixlpark/pixlpark/service"
)

func createProvider(name string, calls int32, counter chan<- int) provider {
	var send int32
	return func(ctx context.Context) *Transform {
		t := &Transform{
			Done:    make(chan struct{}, 0),
			ctx:     ctx,
			ppOrder: pp.Order{ID: fmt.Sprintf("%d", send+1)},
		}
		if ctx.Err() != nil {
			//canceled
			t.err = ctx.Err()
			close(t.Done)
			send = 0
			fmt.Printf("%s canceled\n", name)
			return t
		}

		if send >= calls {
			//complited
			t.err = ErrEmptyQueue{errors.New("Complite")}
			close(t.Done)
			send = 0
			fmt.Printf("%s complite (%d)\n", name, send)
			return t
		}
		//fmt.Printf("%s sending %s\n", name, t.ID())
		send++
		go func(t *Transform) {
			time.Sleep(300 * time.Millisecond)
			if ctx.Err() != nil {
				//canceled
				t.err = ctx.Err()
				fmt.Printf("%s canceled\n", name)
			} else {
				fmt.Printf("%s closing %s\n", name, t.ID())
				counter <- 1
			}
			close(t.Done)
		}(t)
		return t
	}
}

type testFactory struct {
	loadNew          provider
	loadRestart      provider
	transformRestart provider
	finalizeRestart  provider
	softErrorRestart provider
	doOrder          provider
}

func (f *testFactory) LoadNew(ctx context.Context) *Transform {
	return f.loadNew(ctx)
}
func (f *testFactory) LoadRestart(ctx context.Context) *Transform {
	return f.loadRestart(ctx)
}
func (f *testFactory) TransformRestart(ctx context.Context) *Transform {
	return f.transformRestart(ctx)
}
func (f *testFactory) FinalizeRestart(ctx context.Context) *Transform {
	return f.finalizeRestart(ctx)
}
func (f *testFactory) DoOrder(ctx context.Context, notused string) *Transform {
	return f.doOrder(ctx)
}
func (f *testFactory) SoftErrorRestart(ctx context.Context) *Transform {
	return f.softErrorRestart(ctx)
}
func (f *testFactory) SetDebug(debug bool) {
	//noop
}

func createFactory(callsPerCycle, cycles int32, counter chan<- int) (Factory, chan struct{}) {
	done := make(chan struct{})
	chCnt := make(chan int, 4*cycles*callsPerCycle)

	f := &testFactory{
		loadNew:          createProvider("LoadNew", callsPerCycle, chCnt),
		loadRestart:      createProvider("LoadRestart", callsPerCycle, chCnt),
		transformRestart: createProvider("TransformRestart", callsPerCycle, chCnt),
		finalizeRestart:  createProvider("FinalizeRestart", callsPerCycle, chCnt),
		softErrorRestart: createProvider("SoftErrorRestart", callsPerCycle, chCnt),
		doOrder:          createProvider("DoOrder", callsPerCycle, chCnt),
	}

	go func() {
		var cyclesDone int32
		var cnt int32
		for cyclesDone < cycles {
			select {
			case <-chCnt:
				cnt++
				if cnt == 4*callsPerCycle {
					cyclesDone++
					cnt = 0
					fmt.Printf("Cycles done %d\n", cyclesDone)
				}
				counter <- 1
			}
		}
		//cycles complited
		fmt.Printf("All cycles commplite\n")
		close(chCnt)
		close(done)
	}()

	return f, done
}

func Test_runQueue(t *testing.T) {

	m := &Manager{
		concurrency: 3,
	}
	var total int32 = 20
	var send int32
	var done int32

	f := func(ctx context.Context) *Transform {
		t := &Transform{
			Done:    make(chan struct{}, 0),
			ctx:     ctx,
			ppOrder: pp.Order{ID: fmt.Sprintf("%d", send+1)},
		}
		if atomic.LoadInt32(&send) >= total {
			//complited
			t.err = ErrEmptyQueue{errors.New("Complite")}
			close(t.Done)
			return t
		}
		atomic.AddInt32(&send, 1)
		go func(t *Transform) {
			time.Sleep(300 * time.Millisecond)
			fmt.Printf("Closing %s\n", t.ID())
			close(t.Done)
			atomic.AddInt32(&done, 1)
		}(t)
		return t
	}

	err := m.runQueue(context.Background(), f, true)
	if err != nil {
		t.Errorf("Unexpected error %s", err)
	}

	if send != total {
		t.Errorf("Expected sends %d got %d", total, send)
	}
	if done != int32(total) {
		t.Errorf("Expected done %d got %d", total, done)
	}
	if m.transforms == nil {
		t.Errorf("Monitor not started")
	} else if len(m.transforms) != 0 {
		t.Errorf("Monitor is not clean; len = %d", len(m.transforms))
	}
}

func Test_cancelQueue(t *testing.T) {
	m := &Manager{
		concurrency: 2,
	}
	var calls int32 = 20
	var doneCalls int32
	chCounter := make(chan int, calls)

	p := createProvider("Provider", calls, chCounter)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Second, func() {
		cancel()
		fmt.Printf("Context canceled\n")
	})
	err := m.runQueue(ctx, p, false)
	close(chCounter)
	for range chCounter {
		doneCalls++
	}

	if err == nil {
		t.Errorf("Expect error 'context canceled' got nil")
	}
	if doneCalls == calls {
		t.Errorf("runQueue not canceled")
	}
}

func Test_runManager(t *testing.T) {
	var calls int32 = 3
	var cycles int32 = 5
	var doneCalls int32

	chCounter := make(chan int, 4*cycles*calls)
	f, chDone := createFactory(calls, cycles, chCounter)

	m := NewManager(f, 3, 5, nil)
	//3ces interval
	m.interval = 3

	go func() {
		select {
		case <-chDone:
			fmt.Printf("Quiting manager\n")
			m.Quit()
		}
	}()

	start := time.Now()
	m.Start()
	fmt.Printf("Manager started\n")
	m.Wait()
	runSec := time.Since(start).Seconds()
	close(chCounter)
	fmt.Printf("Manager complite\n")
	for range chCounter {
		doneCalls++
	}

	if doneCalls != 4*cycles*calls {
		t.Errorf("Expected calls %d got %d", 4*cycles*calls, doneCalls)
	}
	expecT := float64((int(cycles)-1)*m.interval) + 0.3*float64(cycles)
	if runSec < expecT {
		t.Errorf("Expected run time > %.2f got %.2f", expecT, runSec)
	}

}

func Test_quitManager(t *testing.T) {
	var calls int32 = 10
	var cycles int32 = 20
	var doneCalls int32

	//quit while running
	chCounter := make(chan int, 4*cycles*calls)
	f, chDone := createFactory(calls, cycles, chCounter)
	m := NewManager(f, 1, 5, nil)
	//10ces interval
	m.interval = 10
	go func() {
		select {
		case <-chDone:
			t.Errorf("Unexpected manager quit")
			m.Quit()
		}
	}()
	start := time.Now()
	m.Start()
	fmt.Printf("Manager started\n")
	time.AfterFunc(time.Second, func() {
		m.Quit()
		fmt.Printf("Quit while running done\n")
	})
	m.Wait()
	runSec := time.Since(start).Seconds()
	close(chCounter)
	for range chCounter {
		doneCalls++
	}
	if runSec > 2.0 {
		t.Errorf("Manager not quit while running. Expected run time < %.2fs got %.2f", 2.0, runSec)
	}
	if doneCalls == 4*cycles*calls {
		t.Errorf("Manager not quit while running. Expected calls < %d", 4*cycles*calls)
	}

	//quit while pause
	calls = 1
	chCounter = make(chan int, 4*cycles*calls)
	f, chDone = createFactory(calls, cycles, chCounter)
	m = NewManager(f, 1, 5, nil)
	//10ces interval
	m.interval = 10
	go func() {
		select {
		case <-chDone:
			t.Errorf("Unexpected manager quit")
			m.Quit()
		}
	}()
	start = time.Now()
	m.Start()
	fmt.Printf("Manager started\n")
	time.AfterFunc(3*time.Second, func() {
		if m.IsRunning() {
			t.Errorf("Manager is running, you missed")
		}
		m.Quit()
		fmt.Printf("Quit while pause done\n")
	})
	m.Wait()
	runSec = time.Since(start).Seconds()
	close(chCounter)
	for range chCounter {
		doneCalls++
	}
	if runSec > 4 {
		t.Errorf("Manager not quit while paused. Expected run time < %.2fs got %.2f", 4.0, runSec)
	}
	if doneCalls == 4*cycles*calls {
		t.Errorf("Manager not quit while paused. Expected calls < %d", 4*cycles*calls)
	}

}

func Test_pauseManager(t *testing.T) {
	var calls int32 = 3
	var cycles int32 = 5
	var doneCalls int32

	chCounter := make(chan int, 4*cycles*calls)
	f, chDone := createFactory(calls, cycles, chCounter)

	m := NewManager(f, 1, 5, nil)
	//3ces interval
	m.interval = 3

	go func() {
		select {
		case <-chDone:
			fmt.Printf("Quiting manager\n")
			m.Quit()
		}
	}()

	paused1 := false
	paused2 := false
	//pause while sleep
	m.Start()
	fmt.Printf("Manager started\n")
	time.AfterFunc(4*time.Second, func() {
		m.Pause()
		fmt.Printf("Paused 1 done\n")
	})
	time.AfterFunc(4*time.Second+100*time.Millisecond, func() {
		paused1 = !m.IsRunning()
		m.Start()
		fmt.Printf("Manager started\n")
		//pause while running
		time.AfterFunc(500*time.Millisecond, func() {
			m.Pause()
			fmt.Printf("Manager paused 2\n")
		})
		time.AfterFunc(800*time.Millisecond, func() {
			paused2 = !m.IsRunning()
			fmt.Printf("Manager quit\n")
			m.Quit()
		})
	})
	m.Wait()

	close(chCounter)
	fmt.Printf("Manager complite\n")
	for range chCounter {
		doneCalls++
	}

	if !paused1 {
		t.Errorf("Manager not paused while sleeping")
	}
	if !paused2 {
		t.Errorf("Manager not paused while running")
	}
}
