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
		if send >= calls {
			//complited
			t.err = ErrEmptyQueue{errors.New("Complite")}
			close(t.Done)
			send = 0
			fmt.Printf("%s complite (%d)\n", name, send)
			return t
		}
		fmt.Printf("%s sending %s\n", name, t.ID())
		send++
		go func(t *Transform) {
			time.Sleep(100 * time.Millisecond)
			fmt.Printf("%s closing %s\n", name, t.ID())
			counter <- 1
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

func createFactory(callsPerCycle, cycles int32, counter chan<- int) (Factory, chan struct{}) {
	done := make(chan struct{})
	chCnt := make(chan int, 4*cycles*callsPerCycle)

	f := &testFactory{
		loadNew:          createProvider("LoadNew", callsPerCycle, chCnt),
		loadRestart:      createProvider("LoadRestart", callsPerCycle, chCnt),
		transformRestart: createProvider("TransformRestart", callsPerCycle, chCnt),
		finalizeRestart:  createProvider("FinalizeRestart", callsPerCycle, chCnt),

		doOrder: createProvider("DoOrder", callsPerCycle, chCnt),
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
			fmt.Printf("Closing %s; monitor %d\n", t.ID(), len(m.transforms))
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

	if runSec < float64((int(cycles)-1)*m.interval) {
		t.Errorf("Expected run time > %d got %.2f", ((int(cycles) - 1) * m.interval), runSec)
	}

}
