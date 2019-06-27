package service

import (
	"context"
	"encoding/json"
	"fmt"
	http1 "net/http"
	"net/url"
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

	return Endpoints{
		CountOrdersEndpoint: countOrdersEndpoint,
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

func statusError(code int) error {
	return fmt.Errorf("Wrong http status %d. %s", code, http1.StatusText(code))
}

func copyURL(base *url.URL, path string) (next *url.URL) {
	n := *base
	n.Path = path
	next = &n
	return
}
