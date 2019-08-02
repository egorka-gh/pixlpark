package dispatcher

import (
	"sync/atomic"

	pc "github.com/egorka-gh/pixlpark/photocycle"
	pp "github.com/egorka-gh/pixlpark/pixlpark/service"
)

//Dispatcher directs orders flow between pixelpark and photocycle
type dispatcher struct {
	ppClient  pp.PPService
	pcClient  pc.Repository
	isLoading int64
}

//Run start loading queue and sync pp vs cycle
func (d *dispatcher) Run() {
	if atomic.CompareAndSwapInt64(&d.isLoading, 0, 1) {
		//start loading
	}
	//sync cycle state 199 vs pp.printing set in cycle 200

}
