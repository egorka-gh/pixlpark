package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	http1 "net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/transport/http"
)

// New returns an PPService backed by an HTTP server living at the remote
// instance. We expect instance to come from a service discovery system, so
// likely of the form "host:port".
func New(instance string, options map[string][]http.ClientOption, mdw map[string][]endpoint.Middleware) (PPService, error) {
	if !strings.HasPrefix(instance, "http") {
		instance = "http://" + instance
	}
	u, err := url.Parse(instance)
	if err != nil {
		return nil, err
	}
	var countOrdersEndpoint endpoint.Endpoint
	{
		countOrdersEndpoint = http.NewClient("GET", copyURL(u, "/orders/count"), encodeCountOrdersRequest, decodeCountOrderResponse, options["CountOrders"]...).Endpoint()
		for _, m := range mdw["CountOrders"] {
			countOrdersEndpoint = m(countOrdersEndpoint)
		}
	}
	var getOrdersEndpoint endpoint.Endpoint
	{
		getOrdersEndpoint = http.NewClient("GET", copyURL(u, "/orders"), encodeGetOrdersRequest, decodeGetOrdersResponse, options["GetOrders"]...).Endpoint()
		for _, m := range mdw["GetOrders"] {
			getOrdersEndpoint = m(getOrdersEndpoint)
		}
	}
	var getOrderEndpoint endpoint.Endpoint
	{
		getOrderEndpoint = http.NewClient("GET", copyURL(u, "/orders"), encodeGetOrderRequest, decodeGetOrdersResponse, options["GetOrder"]...).Endpoint()
		for _, m := range mdw["GetOrder"] {
			getOrderEndpoint = m(getOrderEndpoint)
		}
	}
	var getOrderItemsEndpoint endpoint.Endpoint
	{
		getOrderItemsEndpoint = http.NewClient("GET", copyURL(u, "/orders"), encodeGetOrderItemsRequest, decodeGetOrderItemsResponse, options["GetOrderItems"]...).Endpoint()
		for _, m := range mdw["GetOrderItems"] {
			getOrderItemsEndpoint = m(getOrderItemsEndpoint)
		}
	}
	var setOrderStatusEndpoint endpoint.Endpoint
	{
		setOrderStatusEndpoint = http.NewClient("POST", copyURL(u, "/orders"), encodeSetOrderStatusRequest, decodeSetOrderStatusResponse, options["SetOrderStatus"]...).Endpoint()
		for _, m := range mdw["SetOrderStatus"] {
			setOrderStatusEndpoint = m(setOrderStatusEndpoint)
		}
	}

	return Endpoints{
		CountOrdersEndpoint:    countOrdersEndpoint,
		GetOrdersEndpoint:      getOrdersEndpoint,
		GetOrderEndpoint:       getOrderEndpoint,
		GetOrderItemsEndpoint:  getOrderItemsEndpoint,
		SetOrderStatusEndpoint: setOrderStatusEndpoint,
	}, nil
}

func encodeCountOrdersRequest(_ context.Context, r *http1.Request, request interface{}) error {
	req := request.(CountOrdersRequest)
	q := r.URL.Query()
	q.Add("statuses", strings.Join(req.Statuses, ","))
	r.URL.RawQuery = q.Encode()
	return nil
}

func decodeCountOrderResponse(_ context.Context, r *http1.Response) (interface{}, error) {
	if r.StatusCode != http1.StatusOK {
		return nil, statusError(r.StatusCode)
	}
	var resp CountOrdersResponse
	err := json.NewDecoder(r.Body).Decode(&resp)
	return resp, err
}

func encodeGetOrdersRequest(_ context.Context, r *http1.Request, request interface{}) error {
	req := request.(GetOrdersRequest)
	q := r.URL.Query()
	if req.ShippingID != 0 {
		q.Add("shippingId", strconv.Itoa(req.ShippingID))
	}
	if req.Skip != 0 {
		q.Add("skip", strconv.Itoa(req.Skip))
	}
	if req.Status != "" {
		q.Add("status", req.Status)
	}
	if req.Take != 0 {
		q.Add("take", strconv.Itoa(req.Take))
	}
	if req.UserID != 0 {
		q.Add("userId", strconv.Itoa(req.UserID))
	}
	r.URL.RawQuery = q.Encode()
	return nil
}

func decodeGetOrdersResponse(_ context.Context, r *http1.Response) (interface{}, error) {
	if r.StatusCode != http1.StatusOK {
		return nil, statusError(r.StatusCode)
	}
	var resp GetOrdersResponse
	/* to debug response
	var raw bytes.Buffer
	tee := io.TeeReader(r.Body, &raw)
	err := json.NewDecoder(tee).Decode(&resp)
	resp.RawResponse = raw.String()
	fmt.Println(resp.RawResponse)
	*/
	err := json.NewDecoder(r.Body).Decode(&resp)
	return resp, err
}

func encodeGetOrderRequest(_ context.Context, r *http1.Request, request interface{}) error {
	req := request.(GetOrderRequest)
	///orders/{id}/items
	url := copyURL(r.URL, r.URL.Path+"/"+req.ID)
	r.URL = url
	return nil
}

func encodeGetOrderItemsRequest(_ context.Context, r *http1.Request, request interface{}) error {
	req := request.(GetOrderItemsRequest)
	///orders/{id}/items
	url := copyURL(r.URL, r.URL.Path+"/"+req.OrderID+"/items")
	r.URL = url
	return nil
}

func decodeGetOrderItemsResponse(_ context.Context, r *http1.Response) (interface{}, error) {
	if r.StatusCode != http1.StatusOK {
		return nil, statusError(r.StatusCode)
	}
	var resp GetOrderItemsResponse
	/* to debug response
	var raw bytes.Buffer
	tee := io.TeeReader(r.Body, &raw)
	err := json.NewDecoder(tee).Decode(&resp)
	resp.RawResponse = raw.String()
	fmt.Println(resp.RawResponse)
	*/
	err := json.NewDecoder(r.Body).Decode(&resp)
	return resp, err
}

func encodeSetOrderStatusRequest(_ context.Context, r *http1.Request, request interface{}) error {
	req := request.(SetOrderStatusRequest)
	// /orders/{id}/status
	rurl := copyURL(r.URL, r.URL.Path+"/"+req.OrderID+"/status")
	r.URL = rurl
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	form := url.Values{}
	form.Add("newStatus", req.Status)
	if req.Notify {
		form.Add("sendNotifications", "true")
	} else {
		form.Add("sendNotifications", "false")
	}
	r.Body = ioutil.NopCloser(strings.NewReader(form.Encode()))

	/*
		q := r.URL.Query()
		q.Add("newStatus", req.Status)
		if req.Notify {
			q.Add("sendNotifications", "true")
		}
		r.URL.RawQuery = q.Encode()
	*/

	return nil
}

func decodeSetOrderStatusResponse(_ context.Context, r *http1.Response) (interface{}, error) {
	if r.StatusCode != http1.StatusOK {
		return nil, statusError(r.StatusCode)
	}
	var resp SetOrderStatusResponse
	/* to debug response */
	var raw bytes.Buffer
	tee := io.TeeReader(r.Body, &raw)
	err := json.NewDecoder(tee).Decode(&resp)
	resp.RawResponse = raw.String()
	fmt.Println(resp.RawResponse)
	/**/
	//err := json.NewDecoder(r.Body).Decode(&resp)
	return resp, err
}

func statusError(code int) error {
	return fmt.Errorf("Wrong http status %d. %s", code, http1.StatusText(code))
}

func copyURL(base *url.URL, path string) (next *url.URL) {
	n := *base
	n.Path = path
	next = &n
	return
}
