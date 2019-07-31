package main

import (
	"context"
	"fmt"
	http0 "net/http"
	"time"

	"github.com/go-kit/kit/endpoint"
	log "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/transport/http"
)

var methods = []string{"CountOrders", "GetOrders", "GetOrderItems", "GetOrder", "SetOrderStatus", "dummi"}

func defaultHTTPOptions(cli *http0.Client, logger log.Logger) map[string][]http.ClientOption {
	//cli := defaultHttpClient()
	options := map[string][]http.ClientOption{}
	for _, v := range methods {
		options[v] = append(options[v], http.SetClient(cli))
		if logger != nil {
			options[v] = append(options[v], beforeURILogger(log.With(logger, "method", v)))
		}
	}

	//"CountOrders": {http.SetClient(cli), beforeURIExtractor()},
	/*
		"AddActivity": {http.SetClient(cli), clientFinalizer("AddActivity", logger)},
		"GetLevel":    {http.SetClient(cli), clientFinalizer("GetLevel", logger)},
		"ListVersion": {http.SetClient(cli), clientFinalizer("ListVersion", logger)},
		"PackDone":    {http.SetClient(cli), clientFinalizer("PackDone", logger)},
		"PullPack":    {http.SetClient(cli), clientFinalizer("PullPack", logger)},
		"PushPack":    {http.SetClient(cli), clientFinalizer("PushPack", logger)},
	*/

	return options
}

func defaultHTTPMiddleware(logger log.Logger) (mw map[string][]endpoint.Middleware) {
	mw = map[string][]endpoint.Middleware{}

	for _, v := range methods {
		mw[v] = append(mw[v], loggingMiddlware(log.With(logger, "method", v)))
	}
	return
}

func beforeURILogger(l log.Logger) http.ClientOption {
	return http.ClientBefore(
		func(ctx context.Context, r *http0.Request) context.Context {
			l.Log("uri", r.URL.RequestURI())
			return ctx
		},
	)
}

func loggingMiddlware(l log.Logger) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (result interface{}, err error) {
			var req, resp string

			defer func(b time.Time) {
				l.Log(
					"request", req,
					"result", resp,
					"err", err,
					"elapsed", time.Since(b),
				)
			}(time.Now())
			if r, ok := request.(fmt.Stringer); ok {
				req = r.String()
			} else {
				req = fmt.Sprintf("%+v", request)
			}
			result, err = next(ctx, request)
			if r, ok := result.(fmt.Stringer); ok {
				resp = r.String()
			} else {
				resp = fmt.Sprintf("%+v", result)
			}
			return
		}
	}
}

/*
//TODO refactor or remove
func clientFinalizer(method string, logger log.Logger) http.ClientOption {
	lg := log.With(logger, "method", method)
	return http.ClientFinalizer(
		func(ctx context.Context, err error) {
			if err != nil {
				lg.Log("transport_error", err)
			}
		},
	)
}


//TODO create oauth client
//creates transient client, can be pooled?
func defaultHttpClient() *http0.Client {
	cli := &http0.Client{
		Transport: DefaultTransport(),
	}

	cli.Timeout = time.Minute * 3
	return cli
}


// DefaultTransport returns a new http.Transport with similar default values to
// http.DefaultTransport, but with idle connections and keepalives disabled.
func DefaultTransport() *http0.Transport {
	transport := DefaultPooledTransport()
	transport.DisableKeepAlives = true
	transport.MaxIdleConnsPerHost = -1
	return transport
}

// DefaultPooledTransport returns a new http.Transport with similar default
// values to http.DefaultTransport. Do not use this for transient transports as
// it can leak file descriptors over time. Only use this for transports that
// will be re-used for the same host(s).
func DefaultPooledTransport() *http0.Transport {
	transport := &http0.Transport{
		Proxy: http0.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConnsPerHost:   runtime.GOMAXPROCS(0) + 1,
	}
	return transport
}
*/
