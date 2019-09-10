package main

import (
	"context"
	"fmt"
	"os"
	"time"

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

	var oid string
	if len(os.Args) > 1 {
		oid = os.Args[1]
	}
	if oid == "" {
		fmt.Println("Не указан номер заказа")
		return

	}

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
		fmt.Println("Не задано ID источника")
		return
	}

	logger := initLoger(viper.GetString("folders.log"))

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

	url := "http://api.pixlpark.com"
	oauthClient := cnf.Client(context.Background(), nil)
	ttClient, _ := service.New(url, defaultHTTPOptions(oauthClient, nil), defaultHTTPMiddleware(log.With(logger, "level", "transport")))

	/* factory test by Order Id */
	rep, err := repo.New(viper.GetString("mysql"))
	if err != nil {
		logger.Log("Open database error", err.Error())
		fmt.Printf("Ошибка подключения к базе данных %s\n", err.Error())
		return
	}
	fc := transform.NewFactory(ttClient, rep, viper.GetInt("source.id"), viper.GetString("folders.zip"), viper.GetString("folders.in"), viper.GetString("pixelpark.user"), log.With(logger, "level", "factory"))
	fc.SetDebug(true)
	//oid := "1874839**"
	fmt.Printf("Страт заказа %s\n", oid)
	transf := fc.DoOrder(context.Background(), oid)

	t := time.NewTicker(3 * time.Second)
Loop:
	for {
		select {
		case <-t.C:
			fmt.Printf("Скорость загрузки %.2fmb/s\n", transf.BytesPerSecond()/(1024*1024))
		case <-transf.Done:
			// download is complete
			break Loop
		}
	}
	t.Stop()
	err = transf.Err()
	if err != nil {
		logger.Log("TransformError", err.Error())
		fmt.Printf("Ошибка обработки заказа %s\n", err.Error())
	} else {
		fmt.Printf("Заказ обработан, cycle id %s\n", transf.CycleID())
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
		path = path + "order.log"
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
}
