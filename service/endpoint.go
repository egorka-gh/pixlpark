package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kit/kit/endpoint"
)

// Endpoints collects all of the endpoints that compose a profile service. It's
// meant to be used as a helper struct, to collect all of the endpoints into a
// single parameter.
type Endpoints struct {
	CountOrdersEndpoint    endpoint.Endpoint
	GetOrdersEndpoint      endpoint.Endpoint
	GetOrderItemsEndpoint  endpoint.Endpoint
	GetOrderEndpoint       endpoint.Endpoint
	SetOrderStatusEndpoint endpoint.Endpoint
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
	ErrorMessage string `json:"ErrorMessage"`
	RawResponse  string `json:"-"`
}

func (b basePPResponse) check() error {
	if b.APIVersion != APIVersion {
		return fmt.Errorf("Wrong API version. Expected %s. Got %s", APIVersion, b.APIVersion)
	}
	if b.ResponseCode != 200 {
		if b.ErrorMessage == "" {
			return fmt.Errorf("Wrong ResponseCode %d", b.ResponseCode)
		} else {
			return fmt.Errorf("ResponseCode: %d; ErrorMessage: %s", b.ResponseCode, b.ErrorMessage)
		}
	}
	return nil
}

/*
func (b basePPResponse) checkAPIVersion() error {
	if b.APIVersion != APIVersion {
		return fmt.Errorf("Wrong API version. Expected %s. Got %s", APIVersion, b.APIVersion)
	}
	return nil
}
*/

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
	if err = resp.check(); err != nil {
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
	if err = resp.check(); err != nil {
		return []Order{}, err
	}
	return resp.Result, nil
}

//*************** GetOrder

//GetOrderRequest collects the request parameters for the GetOrder method.
type GetOrderRequest struct {
	ID string
}

// GetOrder implements Service. Primarily useful in a client.
func (e Endpoints) GetOrder(ctx context.Context, id string) (Order, error) {
	request := GetOrderRequest{ID: id}
	response, err := e.GetOrderEndpoint(ctx, request)
	if err != nil {
		return Order{}, err
	}
	resp := response.(GetOrdersResponse)
	if err = resp.check(); err != nil {
		return Order{}, err
	}
	if len(resp.Result) != 1 {
		if len(resp.Result) == 0 {
			err = fmt.Errorf("Order id=%s not found", id)
		} else {
			err = fmt.Errorf("Wrong Result len. Expected 1 item. Got %d", len(resp.Result))
		}
		return Order{}, err
	}
	return resp.Result[0], nil
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
	if err = resp.check(); err != nil {
		return []OrderItem{}, err
	}
	return resp.Result, nil
}

//*************** SetOrderStatus

//SetOrderStatusRequest collects the request parameters for the GetOrderItems method.
type SetOrderStatusRequest struct {
	OrderID string
	Status  string
	Notify  bool
}

// SetOrderStatusResponse collects the response parameters for the GetOrderItems method.
type SetOrderStatusResponse struct {
	basePPResponse
	Result []setOrderStatusResult `json:"Result"`
}

type setOrderStatusResult struct {
	Type        string `json:"Type"`
	Description string `json:"Description"`
	DateCreated string `json:"DateCreated"`
}

// SetOrderStatus implements Service. Primarily useful in a client.
func (e Endpoints) SetOrderStatus(ctx context.Context, id, status string, notify bool) error {
	request := SetOrderStatusRequest{OrderID: id, Status: status, Notify: notify}
	response, err := e.SetOrderStatusEndpoint(ctx, request)
	if err != nil {
		return err
	}
	resp := response.(SetOrderStatusResponse)
	if err = resp.check(); err != nil {
		return err
	}
	if len(resp.Result) != 1 {
		return fmt.Errorf("Wrong Result len. Expected 1 item. Got %d", len(resp.Result))
	}
	if strings.ToLower(resp.Result[0].Type) != "success" {
		return fmt.Errorf("Set status error: Type:%s; Description:%s", resp.Result[0].Type, resp.Result[0].Description)
	}
	return nil
}
