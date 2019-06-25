package main

import (
	"context"
	"time"

	http0 "net/http"

	log "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/transport/http"
	cleanhttp "github.com/hashicorp/go-cleanhttp"
)

func defaultHttpOptions(logger log.Logger) map[string][]http.ClientOption {
	cli := defaultHttpClient()
	options := map[string][]http.ClientOption{
		"AddActivity": {http.SetClient(cli), clientFinalizer("AddActivity", logger)},
		"GetLevel":    {http.SetClient(cli), clientFinalizer("GetLevel", logger)},
		"ListVersion": {http.SetClient(cli), clientFinalizer("ListVersion", logger)},
		"PackDone":    {http.SetClient(cli), clientFinalizer("PackDone", logger)},
		"PullPack":    {http.SetClient(cli), clientFinalizer("PullPack", logger)},
		"PushPack":    {http.SetClient(cli), clientFinalizer("PushPack", logger)},
	}
	return options
}

//TODO create oauth client
//creates transient client, can be pooled?
func defaultHttpClient() *http0.Client {
	cli := cleanhttp.DefaultClient()
	cli.Timeout = time.Minute * 3
	return cli
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
