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
	CountOrdersEndpoint endpoint.Endpoint
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
}

func (b basePPResponse) checAPIVersion() error {
	if b.APIVersion != APIVersion {
		return fmt.Errorf("Wrong API version. Expected %s. Got %s", APIVersion, b.APIVersion)
	}
	return nil
}

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
	if err = resp.checAPIVersion(); err != nil {
		return
	}
	if len(resp.Result) != 1 {
		return 0, fmt.Errorf("Wrong Result len. Expected 1 item. Got %d", len(resp.Result))
	}
	return resp.Result[0].Count, nil
}
