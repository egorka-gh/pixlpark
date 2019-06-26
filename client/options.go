package main

import (
	"context"
	http0 "net/http"

	log "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/transport/http"
)

func defaultHttpOptions(cli *http0.Client, logger log.Logger) map[string][]http.ClientOption {
	//cli := defaultHttpClient()
	options := map[string][]http.ClientOption{
		"CountOrders": {http.SetClient(cli)},
		/*
			"AddActivity": {http.SetClient(cli), clientFinalizer("AddActivity", logger)},
			"GetLevel":    {http.SetClient(cli), clientFinalizer("GetLevel", logger)},
			"ListVersion": {http.SetClient(cli), clientFinalizer("ListVersion", logger)},
			"PackDone":    {http.SetClient(cli), clientFinalizer("PackDone", logger)},
			"PullPack":    {http.SetClient(cli), clientFinalizer("PullPack", logger)},
			"PushPack":    {http.SetClient(cli), clientFinalizer("PushPack", logger)},
		*/
	}
	return options
}

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

/*
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
