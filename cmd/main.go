package main

import (
	"context"
	"fmt"
	"os"

	"github.com/egorka-gh/pixlpark/photocycle/repo"
	"github.com/egorka-gh/pixlpark/pixlpark/oauth"
	"github.com/egorka-gh/pixlpark/pixlpark/service"
	"github.com/egorka-gh/pixlpark/transform"
	log "github.com/go-kit/kit/log"
	_ "github.com/go-sql-driver/mysql"
	"github.com/kardianos/osext"
	"github.com/spf13/viper"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

func main() {

	if err := readConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			fmt.Println("Start using default setings")
		} else {
			fmt.Println(err.Error())
			return
		}
	}

	//TODO check settings
	if viper.GetInt("source.id") == 0 {
		fmt.Println("Source ID is not set")
		return
	}

	logger := initLoger(viper.GetString("folders.log"))
	/* */
	//http://api.pixlpark.com
	cnf := &oauth.Config{
		PublicKey:  viper.GetString("pixelpark.oauth.PublicKey"),
		PrivateKey: viper.GetString("pixelpark.oauth.PrivateKey"),
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

	/* factory test by Order Id */
	rep, err := repo.New(viper.GetString("mysql"))
	if err != nil {
		logger.Log("Open database error", err.Error())
		return
	}
	fc := transform.NewFactory(ttClient, rep, viper.GetInt("source.id"), viper.GetString("folders.zip"), viper.GetString("folders.in"), viper.GetString("pixelpark.user"), log.With(logger, "thread", "factory"))
	oid := "1874839&&"
	transf := fc.DoOrder(context.Background(), oid)
	err = transf.Err()
	if err != nil {
		logger.Log("Transform error", err.Error())
	}

	/* factory test
		rep, err := repo.New("root:3411@tcp(127.0.0.1:3306)/fotocycle_cycle?parseTime=true")
		if err != nil {
			logger.Log("Open database error", err.Error())
			return
		}
		fc := transform.NewFactory(ttClient, rep, 23, "D:\\Buffer\\pp\\wrk", "D:\\Buffer\\pp\\res", "photo.cycle@yandex.by", log.With(logger, "thread", "factory"))

		transf := &transform.Transform{}
		transf = fc.ResetStarted(context.Background())
		transf = fc.Do(context.Background())

		// start UI loop
		t := time.NewTicker(500 * time.Millisecond)
	Loop:
		for {
			select {
			case <-t.C:
				fmt.Printf("  download speed %v\n", transf.BytesPerSecond())
			case <-transf.Done:
				// download is complete
				break Loop
			}
		}
		t.Stop()

		err = transf.Err()
		if err != nil {
			logger.Log("Transform error", err.Error())
		}
	*/

	/*
		oid := "1845051"
		items, err := ttClient.GetOrderItems(context.Background(), oid)
		if err != nil {
			logger.Log("OrderItemsId", oid, "GetOrderItems error", err.Error())
		} else {
			logger.Log("OrderItemsId", oid, "Responce", fmt.Sprintf("%+v", items))
		}
	*/

	/*
		err := ttClient.SetOrderStatus(context.Background(), "1850708", "ReadyToProcessing", false) //DesignCoordination //ReadyToProcessing
		//TODO double array in success response
		//TODO warning {"ApiVersion":"1.0","Result":[[{"Type":"Warning","Description":"Заказ уже находится в этом статусе.","DateCreated":"31.07.2019 15:04"}]],"ResponseCode":200}
		//TODO api  can set any state?
		//err response {"ApiVersion":"1.0","ErrorMessage":"Not found","ResponseCode":404}
		if err != nil {
			logger.Log("SetOrderStatus error", err.Error())
		}
	*/

	/*
		o, err := ttClient.GetOrder(context.Background(), "1850708")
		if err != nil {
			logger.Log("GetOrders error", err.Error())
		} else {
			logger.Log("Responce", fmt.Sprintf("%+v", o))
		}
	*/

	/*
		err := ttClient.AddOrderComment(context.Background(), "1850708", "egorka@tut.by", "test add comment 2")
		if err != nil {
			logger.Log("AddOrderComment error", err.Error())
		}
	*/
	/*
		loadZip := false
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
			if loadZip {
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
			}
		}
	*/

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

//ReadConfig init/read viper config
func readConfig() error {

	viper.SetDefault("mysql", "root:3411@tcp(127.0.0.1:3306)/fotocycle_cycle?parseTime=true") //MySQL connection string
	viper.SetDefault("source.id", 23)                                                         //photocycle source id
	viper.SetDefault("folders.zip", "D:\\Buffer\\pp\\wrk")                                    //work folder for loaded  and unpacked zips
	viper.SetDefault("folders.in", "D:\\Buffer\\ftp\\in\\PXP")                                //cycle work folder (in ftp)
	viper.SetDefault("folders.prn", "")                                                       //cycle print folder (out)
	viper.SetDefault("folders.log", "")                                                       //Log folder
	viper.SetDefault("pixelpark.user", "photo.cycle@yandex.by")                               //pixelpark user email to post messages to api
	viper.SetDefault("pixelpark.oauth.PublicKey", "aac2028cc33c4970b9e1a829ca7acd7b")         //oauth PublicKey
	viper.SetDefault("pixelpark.oauth.PrivateKey", "0227f3943b214603b7fa9431a09b325d")        //oauth PrivateKey

	path, err := osext.ExecutableFolder()
	if err != nil {
		path = "."
	}
	//fmt.Println("Path ", path)
	viper.AddConfigPath(path)
	viper.SetConfigName("config")
	return viper.ReadInConfig()
	/*
		if err != nil {
			logger.Info(err)
			logger.Info("Start using default setings")
		}
	*/
}
