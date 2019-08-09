package transform

import (
	pp "github.com/egorka-gh/pixlpark/pixlpark/service"
)

const (
	statePixelStartLoad    = pp.StateReadyToProcessing
	statePixelLoadStarted  = pp.StatePrepressCoordination
	statePixelWaiteConfirm = pp.StatePrepressCoordinationAwaitingReply
	statePixelConfirmed    = pp.StatePrepressCoordinationComplete

	statePixelAbort = pp.StatePrintedWithDefect
)
