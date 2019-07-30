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
	/* */
	//http://api.pixlpark.com
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
	/* */

	/*
		//http://api.staging.pixlpark.com
		cnf := &oauth.Config{
			PublicKey:  "2ef2d5233fcc49bba387a51aabefb678",
			PrivateKey: "24a1aad1c8364d87ae1dde60c8be6dbc",
			Endpoint: oauth.Endpoint{
				RequestURL: "http://api.staging.pixlpark.com/oauth/requesttoken",
				RefreshURL: "http://api.staging.pixlpark.com/oauth/refreshtoken",
				TokenURL:   "http://api.staging.pixlpark.com/oauth/accesstoken",
			},
			//Logger: logger,
		}
	*/

	url := "http://api.pixlpark.com"
	//url := "http://api.staging.pixlpark.com"
	oauthClient := cnf.Client(context.Background(), nil)
	//ttClient, _ := service.New(url, defaultHTTPOptions(oauthClient, logger), defaultHTTPMiddleware(logger))
	ttClient, _ := service.New(url, defaultHTTPOptions(oauthClient, nil), defaultHTTPMiddleware(logger))

	orders, err := ttClient.GetOrders(context.Background(), "", 0, 0, 0, 0)
	if err != nil {
		logger.Log("GetOrders error", err.Error())
	} else {
		logger.Log("Responce", fmt.Sprintf("%+v", orders))

		for _, o := range orders {
			items, err := ttClient.GetOrderItems(context.Background(), o.ID)
			if err != nil {
				logger.Log("OrderItemsId", o.ID, "GetOrderItems error", err.Error())
			} else {
				logger.Log("OrderItemsId", o.ID, "Responce", fmt.Sprintf("%+v", items))
			}
		}
		/*
			//try to load
			loader := grab.NewClient()
			wrkFolder := "D:\\Buffer\\tmp\\"
			for _, o := range orders {
				req, _ := grab.NewRequest(wrkFolder+o.ID+".zip", o.DownloadLink)

				// start download
				fmt.Printf("Downloading %v...\n", req.URL())
				resp := loader.Do(req)
				fmt.Printf("  %v\n", resp.HTTPResponse.Status)

				// start UI loop
				t := time.NewTicker(500 * time.Millisecond)
				//defer t.Stop()

			Loop:
				for {
					select {
					case <-t.C:
						fmt.Printf("  transferred %v / %v bytes (%.2f%%)\n",
							resp.BytesComplete(),
							resp.Size(),
							100*resp.Progress())

					case <-resp.Done:
						// download is complete
						break Loop
					}
				}
				t.Stop()

				// check for errors
				if err := resp.Err(); err != nil {
					fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
					//os.Exit(1)
				}

				fmt.Printf("Download saved to ./%v \n", resp.Filename)
			}
		*/
	}
	//
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
