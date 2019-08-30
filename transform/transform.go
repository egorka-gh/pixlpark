package transform

import (
	"context"
	"strings"
	"time"

	"github.com/cavaliercoder/grab"

	pc "github.com/egorka-gh/pixlpark/photocycle"
	pp "github.com/egorka-gh/pixlpark/pixlpark/service"
)

// Transform represents the transform of pixlpark oreder to photocycle orders.
type Transform struct {
	//PP state in which the order must be fetched
	fetchState string

	//ppOrder pixelpark original order
	ppOrder pp.Order

	//pcBaseOrder dummy photocycle order to store basic states and logs in photocycle database
	pcBaseOrder pc.Order

	//pcOrders photocycle orders transform result, pixelpark order items transformed to photocycle orders
	//pcOrders []pc.Order

	// Start specifies the time at which the file transfer started.
	Start time.Time

	// End specifies the time at which the file transfer completed.
	//
	// This will return zero until the transfer has completed.
	End time.Time

	// Done is closed once the transfer is finalized, either successfully or with
	// errors. Errors are available via Response.Err
	Done chan struct{}

	// ctx is a Context that controls cancelation of an inprogress transfer
	ctx context.Context

	// cancel is a cancel func that can be used to cancel the context of this
	// Response.
	cancel context.CancelFunc

	// loader grab loader
	loader *grab.Response

	// Error contains any error that may have occurred during the file transfer.
	// This should not be read until IsComplete returns true.
	err error
}

// ID returns pp order id
func (t *Transform) ID() string {
	return t.ppOrder.ID
}

//CycleID returns cycle orders id prefix
func (t *Transform) CycleID() string {
	l := len(t.pcBaseOrder.SourceID)
	if l == 0 {
		return ""
	}
	return strings.TrimRight(t.pcBaseOrder.ID, "@")
}

// IsComplete returns true if the transform has completed.
// If an error occurred it can be returned via Err.
func (t *Transform) IsComplete() bool {
	select {
	case <-t.Done:
		return true
	default:
		return false
	}
}

// Cancel cancels the transform by canceling the underlying Context for
// this Transform. Cancel blocks until the transform is closed and returns any
// error - typically context.Canceled.
//TODO recheck, if there is no cancelabel context will blocks until done
func (t *Transform) Cancel() error {
	if t.cancel != nil {
		t.cancel()
	}
	return t.Err()
}

// Wait blocks until the transform is completed.
func (t *Transform) Wait() {
	<-t.Done
}

// Err blocks the calling goroutine until the transform is
// completed and returns any error that may have occurred. If the transform is
// already completed, Err returns immediately.
func (t *Transform) Err() error {
	<-t.Done
	return t.err
}

// BytesPerSecond returns zip loader speed, if it started and still running
// the number of bytes per second transferred using a
// simple moving average of the last five seconds.
func (t *Transform) BytesPerSecond() float64 {
	if t.loader != nil {
		return t.loader.BytesPerSecond()
	}
	return 0
}
