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

func Test_runQueue(t *testing.T) {

	m := &Manager{
		concurrency: 3,
	}
	total := 50
	send := 0
	var done int32

	f := func(ctx context.Context) *Transform {
		t := &Transform{
			Done:    make(chan struct{}, 0),
			ctx:     ctx,
			ppOrder: pp.Order{ID: fmt.Sprintf("%d", send+1)},
		}
		if send >= total {
			//complited
			t.err = ErrEmptyQueue{errors.New("Complite")}
			close(t.Done)
			return t
		}
		send++
		go func(t *Transform) {
			time.Sleep(1 * time.Second)
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
