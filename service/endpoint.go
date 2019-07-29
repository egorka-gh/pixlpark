package service

import (
	"context"
	"fmt"

	"github.com/go-kit/kit/endpoint"
)

// Endpoints collects all of the endpoints that compose a profile service. It's
// meant to be used as a helper struct, to collect all of the endpoints into a
// single parameter.
type Endpoints struct {
	CountOrdersEndpoint   endpoint.Endpoint
	GetOrdersEndpoint     endpoint.Endpoint
	GetOrderItemsEndpoint endpoint.Endpoint
}

/* server version??
func New(s service.ZsyncService, mdw map[string][]endpoint.Middleware) Endpoints {
	eps := Endpoints{
		CountOrdersEndpoint: MakeCountOrdersEndpoint(s),
	}
	for _, m := range mdw["CountOrders"] {
		eps.CountOrdersEndpoint = m(eps.CountOrdersEndpoint)
	}
	return eps
}
*/

//common fields in service responce
type basePPResponse struct {
	APIVersion   string `json:"ApiVersion"`
	ResponseCode int    `json:"ResponseCode"`
	RawResponse  string `json:"-"`
}

func (b basePPResponse) checkAPIVersion() error {
	if b.APIVersion != APIVersion {
		return fmt.Errorf("Wrong API version. Expected %s. Got %s", APIVersion, b.APIVersion)
	}
	return nil
}

//*************** CountOrders

// CountOrdersRequest collects the request parameters for the CountOrders method.
type CountOrdersRequest struct {
	Statuses []string
}

// CountOrders is CountOrdersResponse Result item
type CountOrders struct {
	Count int `json:"count"`
}

// CountOrdersResponse collects the response parameters for the CountOrders method.
type CountOrdersResponse struct {
	basePPResponse
	Result []CountOrders `json:"Result"`
}

// CountOrders implements Service. Primarily useful in a client.
func (e Endpoints) CountOrders(ctx context.Context, statuses []string) (count int, err error) {
	request := CountOrdersRequest{Statuses: statuses}
	response, err := e.CountOrdersEndpoint(ctx, request)
	if err != nil {
		return
	}
	resp := response.(CountOrdersResponse)
	if err = resp.checkAPIVersion(); err != nil {
		return
	}
	if len(resp.Result) != 1 {
		return 0, fmt.Errorf("Wrong Result len. Expected 1 item. Got %d", len(resp.Result))
	}
	return resp.Result[0].Count, nil
}

//*************** GetOrders

//GetOrdersRequest collects the request parameters for the GetOrders method.
type GetOrdersRequest struct {
	Take       int
	Skip       int
	Status     string
	UserID     int
	ShippingID int
}

// GetOrdersResponse collects the response parameters for the CountOrders method.
type GetOrdersResponse struct {
	basePPResponse
	Result []Order `json:"Result"`
}

// GetOrders implements Service. Primarily useful in a client.
func (e Endpoints) GetOrders(ctx context.Context, status string, userID, shippingID, take, skip int) ([]Order, error) {
	request := GetOrdersRequest{Take: take, Skip: skip, Status: status, UserID: userID, ShippingID: shippingID}
	response, err := e.GetOrdersEndpoint(ctx, request)
	if err != nil {
		return []Order{}, err
	}
	resp := response.(GetOrdersResponse)
	if err = resp.checkAPIVersion(); err != nil {
		return []Order{}, err
	}
	return resp.Result, nil
}

//*************** GetOrderItems

//GetOrderItemsRequest collects the request parameters for the GetOrderItems method.
type GetOrderItemsRequest struct {
	OrderID string
}

// GetOrderItemsResponse collects the response parameters for the GetOrderItems method.
type GetOrderItemsResponse struct {
	basePPResponse
	Result []OrderItem `json:"Result"`
}

// GetOrderItems implements Service. Primarily useful in a client.
func (e Endpoints) GetOrderItems(ctx context.Context, orderID string) ([]OrderItem, error) {
	request := GetOrderItemsRequest{OrderID: orderID}
	response, err := e.GetOrderItemsEndpoint(ctx, request)
	if err != nil {
		return []OrderItem{}, err
	}
	resp := response.(GetOrderItemsResponse)
	if err = resp.checkAPIVersion(); err != nil {
		return []OrderItem{}, err
	}
	return resp.Result, nil
}
