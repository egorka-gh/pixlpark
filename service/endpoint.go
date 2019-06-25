package service

import (
	"context"
	"net/url"
	"strings"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/transport/http"
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

// GetLevel implements Service. Primarily useful in a client.
func (e Endpoints) CountOrders(ctx context.Context, statuses []string) (i0 int, e1 error) {
	/*
		request := GetLevelRequest{Card: card}
		response, err := e.GetLevelEndpoint(ctx, request)
		if err != nil {
			return
		}
		return response.(GetLevelResponse).I0, response.(GetLevelResponse).E1
	*/
	return 10, nil
}

// New returns an PPService backed by an HTTP server living at the remote
// instance. We expect instance to come from a service discovery system, so
// likely of the form "host:port".
func New(instance string, options map[string][]http.ClientOption) (PPService, error) {
	if !strings.HasPrefix(instance, "http") {
		instance = "http://" + instance
	}
	u, err := url.Parse(instance)
	if err != nil {
		return nil, err
	}
	var countOrdersEndpoint endpoint.Endpoint
	{
		//countOrdersEndpoint = http.NewClient("POST", copyURL(u, "/list-version"), encodeHTTPGenericRequest, decodeListVersionResponse, options["ListVersion"]...).Endpoint()
	}

	return Endpoints{
		CountOrdersEndpoint: countOrdersEndpoint,
	}, nil
}

func copyURL(base *url.URL, path string) (next *url.URL) {
	n := *base
	n.Path = path
	next = &n
	return
}
