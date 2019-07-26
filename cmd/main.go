package main

import (
	"context"
	"fmt"
	"os"

	"github.com/egorka-gh/pixlpark/oauth"
	"github.com/egorka-gh/pixlpark/service"
	log "github.com/go-kit/kit/log"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

func main() {

	logger := initLoger("")
	cnf := &oauth.Config{
		PublicKey:  "aac2028cc33c4970b9e1a829ca7acd7b",
		PrivateKey: "0227f3943b214603b7fa9431a09b325d",
		Endpoint: oauth.Endpoint{
			RequestURL: "http://api.pixlpark.com/oauth/requesttoken",
			RefreshURL: "http://api.pixlpark.com/oauth/refreshtoken",
			TokenURL:   "http://api.pixlpark.com/oauth/accesstoken",
		},
		//Logger: logger,
	}

	//url := "http://api.pixlpark.com/orders/count"
	url := "http://api.pixlpark.com"
	oauthClient := cnf.Client(context.Background(), nil)
	//ttClient, _ := service.New(url, defaultHTTPOptions(oauthClient, logger), defaultHTTPMiddleware(logger))
	ttClient, _ := service.New(url, defaultHTTPOptions(oauthClient, nil), defaultHTTPMiddleware(logger))

	orders, err := ttClient.GetOrders(context.Background(), "", 0, 0, 0, 0)
	if err != nil {
		logger.Log("GetOrders error", err.Error())
	}
	logger.Log("Responce", fmt.Sprintf("%+v", orders))
	/*
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		q := make(chan os.Signal, 1)
		signal.Notify(q, syscall.SIGINT, syscall.SIGTERM)
		var calls int
		for alive := true; alive; {
			select {
			case <-q:
				alive = false
			case <-ticker.C:
				calls++
				if calls < 3 {
					//calls vsout refresh
					//r, err := c.Get(url)
					count, err := ttClient.CountOrders(context.Background(), []string{})
					if err != nil {
						logger.Log("CountOrders error", err.Error())
					}
					logger.Log("Responce", count)
				} else {
					//alive = false
					//waite till can refresh
					//fist call will refresh
					if calls > 25 {
						calls = 0
					}
				}
				//rerun work
			}
		}

	*/

}

func initLoger(logPath string) log.Logger {
	var logger log.Logger
	if logPath == "" {
		logger = log.NewLogfmtLogger(os.Stderr)
	} else {
		path := logPath
		if !os.IsPathSeparator(path[len(path)-1]) {
			path = path + string(os.PathSeparator)
		}
		path = path + "zsync.log"
		logger = log.NewLogfmtLogger(&lumberjack.Logger{
			Filename:   path,
			MaxSize:    5, // megabytes
			MaxBackups: 5,
			MaxAge:     60, //days
		})
	}
	logger = log.With(logger, "ts", log.DefaultTimestamp) // .DefaultTimestampUTC)
	logger = log.With(logger, "caller", log.DefaultCaller)

	return logger
}
