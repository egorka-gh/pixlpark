package service

//MailPackage represents the MailPackage dto for cycle web client
type MailPackage struct {
	ID            string            `json:"id"`
	IDName        string            `json:"number"`
	ClientID      int               `json:"client_id"`
	ExecutionDate string            `json:"execution_date"`
	DeliveryID    int               `json:"native_delivery_id"`
	DeliveryName  string            `json:"delivery_title"`
	StateName     string            `json:"src_state_name"`
	Properties    map[string]string `json:"address"`
	//TODO messages?
	//TODO barcodes?
}
