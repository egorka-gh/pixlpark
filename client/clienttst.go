package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/egorka-gh/pixlpark/oauth"
	"github.com/egorka-gh/pixlpark/service"
	log "github.com/go-kit/kit/log"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

func main() {

	cnf := &oauth.Config{
		PublicKey:  "2ef2d5233fcc49bba387a51aabefb678",
		PrivateKey: "24a1aad1c8364d87ae1dde60c8be6dbc",
		Endpoint: oauth.Endpoint{
			RequestURL: "http://api.pixlpark.com/oauth/requesttoken",
			RefreshURL: "http://api.pixlpark.com/oauth/refreshtoken",
			TokenURL:   "http://api.pixlpark.com/oauth/accesstoken",
		},
		Logger: initLoger(""),
	}

	//url := "http://api.pixlpark.com/orders/count"
	url := "http://api.pixlpark.com"
	oauthClient := cnf.Client(context.Background(), nil)
	ttClient, _ := service.New(url, defaultHttpOptions(oauthClient, cnf.Logger))

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
					cnf.Logger.Log("CountOrders error", err.Error())
				}
				/*
					bodyBytes, err := ioutil.ReadAll(r.Body)
					if err != nil {
						cnf.Logger.Log("fetching error", err.Error())
					}
					cnf.Logger.Log("Responce", string(bodyBytes))
					//io.Copy(os.Stdout, r.Body)
					r.Body.Close()
				*/
				cnf.Logger.Log("Responce", count)
			} else {
				//alive = false
				//waite till can refresh
				//fist call will refresh
				if calls > 25 {
					calls = 0
				}
				/*
					//waite till can't refresh
					//fist call will fetch
					if calls > 30 {
						calls = 0
					}
				*/
			}
			//rerun work
		}
	}

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
