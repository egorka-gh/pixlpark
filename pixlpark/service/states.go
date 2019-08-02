package service

//StateNone represent service state
const StateNone = "None"

//StatePreparing represent service state
const StatePreparing = "Preparing"

//StateNotProcessed represent service state
const StateNotProcessed = "NotProcessed"

//StateAwaitingPayment represent service state
const StateAwaitingPayment = "AwaitingPayment"

//StateReadyToProcessing represent service state
const StateReadyToProcessing = "ReadyToProcessing"

//StateDesignCoordination represent service state
const StateDesignCoordination = "DesignCoordination"

//StateDesignCoordinationComplete represent service state
const StateDesignCoordinationComplete = "DesignCoordinationComplete"

//StateDesignCoordinationAwaitingReply represent service state
const StateDesignCoordinationAwaitingReply = "DesignCoordinationAwaitingReply"

//StatePrepressCoordination represent service state
const StatePrepressCoordination = "PrepressCoordination"

//StatePrepressCoordinationComplete represent service state
const StatePrepressCoordinationComplete = "PrepressCoordinationComplete"

//StatePrepressCoordinationAwaitingReply represent service state
const StatePrepressCoordinationAwaitingReply = "PrepressCoordinationAwaitingReply"

//StatePrinting represent service state
const StatePrinting = "Printing"

//StatePrintedWithDefect represent service state
const StatePrintedWithDefect = "PrintedWithDefect"

//StatePostPress represent service state
const StatePostPress = "PostPress"

//StatePrinted represent service state
const StatePrinted = "Printed"

//StateShipped represent service state
const StateShipped = "Shipped"

//StateShippedToStorage represent service state
const StateShippedToStorage = "ShippedToStorage"

//StateReturned represent service state
const StateReturned = "Returned"

//StateCancelled represent service state
const StateCancelled = "Cancelled"

//StateCancelledWithDefect represent service state
const StateCancelledWithDefect = "CancelledWithDefect"

//StateRefused represent service state
const StateRefused = "Refused"

//StateDelivered represent service state
const StateDelivered = "Delivered"

//StateDeleted represent service state
// Заказ удален (при переводе заказа в этот статус, заказ просто удаляется из системы)
const StateDeleted = "Deleted"
