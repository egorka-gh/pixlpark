package service

const (
	//StateNone represent service state
	StateNone = "None"

	//StatePreparing represent service state
	StatePreparing = "Preparing"

	//StateNotProcessed represent service state
	StateNotProcessed = "NotProcessed"

	//StateAwaitingPayment represent service state
	StateAwaitingPayment = "AwaitingPayment"

	//StateReadyToProcessing represent service state
	StateReadyToProcessing = "ReadyToProcessing"

	//StateDesignCoordination represent service state
	StateDesignCoordination = "DesignCoordination"

	//StateDesignCoordinationComplete represent service state
	StateDesignCoordinationComplete = "DesignCoordinationComplete"

	//StateDesignCoordinationAwaitingReply represent service state
	StateDesignCoordinationAwaitingReply = "DesignCoordinationAwaitingReply"

	//StatePrepressCoordination represent service state
	StatePrepressCoordination = "PrepressCoordination"

	//StatePrepressCoordinationComplete represent service state
	StatePrepressCoordinationComplete = "PrepressCoordinationComplete"

	//StatePrepressCoordinationAwaitingReply represent service state
	StatePrepressCoordinationAwaitingReply = "PrepressCoordinationAwaitingReply"

	//StatePrinting represent service state
	StatePrinting = "Printing"

	//StatePrintedWithDefect represent service state
	StatePrintedWithDefect = "PrintedWithDefect"

	//StatePostPress represent service state
	StatePostPress = "PostPress"

	//StatePrinted represent service state
	StatePrinted = "Printed"

	//StateShipped represent service state
	StateShipped = "Shipped"

	//StateShippedToStorage represent service state
	StateShippedToStorage = "ShippedToStorage"

	//StateReturned represent service state
	StateReturned = "Returned"

	//StateCancelled represent service state
	StateCancelled = "Cancelled"

	//StateCancelledWithDefect represent service state
	StateCancelledWithDefect = "CancelledWithDefect"

	//StateRefused represent service state
	StateRefused = "Refused"

	//StateDelivered represent service state
	StateDelivered = "Delivered"

	//StateDeleted represent service state
	// Заказ удален (при переводе заказа в этот статус, заказ просто удаляется из системы)
	StateDeleted = "Deleted"
)
