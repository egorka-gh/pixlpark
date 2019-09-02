package main

import (
	"context"
	"fmt"
	http0 "net/http"
	"time"

	"github.com/egorka-gh/pixlpark/pixlpark/service"
	"github.com/go-kit/kit/endpoint"
	log "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/transport/http"
)

var methods = []string{"CountOrders", "GetOrders", "GetOrderItems", "GetOrder", "SetOrderStatus", "AddOrderComment", "dummi"}

func defaultHTTPOptions(cli *http0.Client, logger log.Logger) map[string][]http.ClientOption {
	//cli := defaultHttpClient()
	options := map[string][]http.ClientOption{}
	for _, v := range methods {
		options[v] = append(options[v], http.SetClient(cli))
		if logger != nil {
			options[v] = append(options[v], beforeURILogger(log.With(logger, "method", v)))
		}
		options[v] = append(options[v], beforeSetDebug(true))
	}
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

func beforeSetDebug(debug bool) http.ClientOption {
	return http.ClientBefore(
		func(ctx context.Context, r *http0.Request) context.Context {
			return context.WithValue(ctx, service.HTTPDebug, debug)
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
