package main

import (
	"context"
	"errors"
	"fmt"
	clog "log"
	"net/http"
	"os"
	"path"
	"time"

	cycle "github.com/egorka-gh/pixlpark/photocycle"
	"github.com/egorka-gh/pixlpark/photocycle/repo"
	proxy "github.com/egorka-gh/pixlpark/photocycle/service"
	"github.com/egorka-gh/pixlpark/pixlpark/oauth"
	"github.com/egorka-gh/pixlpark/pixlpark/service"
	"github.com/egorka-gh/pixlpark/transform"
	log "github.com/go-kit/kit/log"
	_ "github.com/go-sql-driver/mysql"
	"github.com/kardianos/osext"
	service1 "github.com/kardianos/service"
	group "github.com/oklog/oklog/pkg/group"

	"github.com/spf13/viper"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

//demon logger
var dLogger service1.Logger

//
type program struct {
	group     *group.Group
	rep       cycle.Repository
	interrupt chan struct{}
	quit      chan struct{}
}

//start os demon or console using kardianos
func main() {
	err := readConfig()
	if err != nil {
		clog.Fatal(err)
		return
	}

	var instanseID = viper.GetString("source.id")
	svcConfig := &service1.Config{
		Name:        "Pixel_" + instanseID,
		DisplayName: "Pixel Service id:" + instanseID,
		Description: "Pixel sub service for PhotoCycle",
	}
	prg := &program{}

	s, err := service1.New(prg, svcConfig)
	if err != nil {
		clog.Fatal(err)
		return
	}
	if len(os.Args) > 1 {
		err = service1.Control(s, os.Args[1])
		if err != nil {
			clog.Fatal(err)
		}
		return
	}
	dLogger, err = s.Logger(nil)
	if err != nil {
		clog.Fatal(err)
	}
	err = s.Run()
	if err != nil {
		dLogger.Error(err)
	}

}

func (p *program) Start(s service1.Service) error {
	g, rep, err := initPixel()
	if err != nil {
		return err
	}

	p.group = g
	p.rep = rep
	p.interrupt = make(chan struct{})
	p.quit = make(chan struct{})

	if service1.Interactive() {
		dLogger.Info("Running in terminal.")
		dLogger.Infof("Valid startup parametrs: %q\n", service1.ControlAction)
	} else {
		dLogger.Info("Starting Pixel service...")
	}
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}

func (p *program) run() {
	//close db cnn
	defer func() {
		if p.rep != nil {
			p.rep.Close()
		}
	}()
	running := make(chan struct{})
	//initCancelInterrupt
	p.group.Add(
		func() error {
			select {
			case <-p.interrupt:
				return errors.New("Pixel: Get interrupt signal")
			case <-running:
				return nil
			}
		}, func(error) {
			close(running)
		})
	dLogger.Info("Pixel started")
	dLogger.Info(p.group.Run())
	close(p.quit)
}

func (p *program) Stop(s service1.Service) error {
	// Stop should not block. Return with a few seconds.
	dLogger.Info("Pixel Stopping!")
	//interrupt service
	close(p.interrupt)
	//waite service stops
	<-p.quit
	dLogger.Info("Pixel stopped")
	return nil
}

func initPixel() (*group.Group, cycle.Repository, error) {
	//TODO check settings
	instanseID := viper.GetInt("source.id")
	if instanseID == 0 {
		return nil, nil, errors.New("Не задано ID источника")
	}
	if viper.GetString("pixelpark.oauth.PublicKey") == "" || viper.GetString("pixelpark.oauth.PrivateKey") == "" {
		return nil, nil, errors.New("Не заданы параметры oauth")
	}
	if viper.GetString("mysql") == "" {
		return nil, nil, errors.New("Не задано подключение mysql")
	}
	if viper.GetString("proxy.address") == "" {
		return nil, nil, errors.New("Не задан host:port для локального сервера")
	}
	//TODO log startup params

	logger := initLoger(viper.GetString("folders.log"), "pixel")

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
	ppClient, _ := service.New(url, defaultHTTPOptions(oauthClient, nil), defaultHTTPMiddleware(log.With(logger, "level", "transport")))

	//create repro
	rep, err := repo.New(viper.GetString("mysql"), false)
	if err != nil {
		logger.Log("Open database error", err.Error())
		return nil, nil, fmt.Errorf("Ошибка подключения к базе данных %s", err.Error())
	}

	//create factory
	fc := transform.NewFactory(ppClient, rep, instanseID, viper.GetInt("production.pixel"), viper.GetString("folders.zip"), viper.GetString("folders.in"), viper.GetString("folders.prn"), viper.GetString("pixelpark.user"), log.With(logger, "level", "factory"))

	//check test mode
	if viper.GetBool("debug") {
		dLogger.Info("Run in debug mode.")
		fc.SetDebug(true)
	}

	//create manager
	mn := transform.NewManager(fc, viper.GetInt("threads"), viper.GetInt("interval"), logger)
	g := &group.Group{}

	//init transform manager
	g.Add(func() error {
		mn.Start()
		mn.Wait()
		return nil
	}, func(error) {
		mn.Quit()
	})

	//init proxy
	//init proxy
	pcfg := proxy.Config{
		PixelClient: ppClient,
		CycleClient: rep,
		Manager:     mn,
		Source:      instanseID,
	}

	server := &http.Server{
		Addr:         viper.GetString("proxy.address"),
		Handler:      proxy.New(&pcfg),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  15 * 60 * time.Second,
	}
	g.Add(func() error {
		//logger.Log("transport", "debug/HTTP", "addr", debugAddr)
		dLogger.Info(fmt.Sprintf("Starting pixel proxy at %s.", server.Addr))
		//dLogger.Info(fmt.Sprintf("Debug endpoint at %s/debug/pprof/.", server.Addr))
		return server.ListenAndServe()
	}, func(error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})

	return g, rep, nil
}

func initLoger(logPath, fileName string) log.Logger {
	var logger log.Logger
	if logPath == "" {
		logger = log.NewLogfmtLogger(os.Stderr)
	} else {
		if fileName == "" {
			fileName = "log"
		}
		p := path.Join(logPath, fmt.Sprintf("%s.log", fileName))
		logger = log.NewLogfmtLogger(&lumberjack.Logger{
			Filename:   p,
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
	viper.SetDefault("production.pixel", 0)                                                   //production pixelparck id (0 - all orders)
	viper.SetDefault("production.cycle", 0)                                                   //corresponding production cycle id
	viper.SetDefault("interval", 10)                                                          //processing interval (min)
	viper.SetDefault("threads", 3)                                                            //processing threads
	viper.SetDefault("folders.zip", "D:\\Buffer\\pp\\wrk")                                    //work folder for loaded  and unpacked zips
	viper.SetDefault("folders.in", "D:\\Buffer\\ftp\\in\\PXP")                                //cycle work folder (in ftp)
	viper.SetDefault("folders.prn", "D:\\Buffer\\ftp\\out\\PXP")                              //cycle print folder (out)
	viper.SetDefault("folders.log", ".\\log")                                                 //Log folder
	viper.SetDefault("pixelpark.user", "photo.cycle@yandex.by")                               //pixelpark user email to post messages to api
	viper.SetDefault("pixelpark.oauth.PublicKey", "aac2028cc33c4970b9e1a829ca7acd7b")         //oauth PublicKey
	viper.SetDefault("pixelpark.oauth.PrivateKey", "0227f3943b214603b7fa9431a09b325d")        //oauth PrivateKey
	viper.SetDefault("proxy.address", ":8888")                                                //localhost
	viper.SetDefault("paperIdMap", map[string]string{"10": "Глянцевая", "11": "Матовая", "12": "Металлик", "13": "Шелк"})
	viper.SetDefault("debug", false)       //set debug mode (will not change pixel orders states)
	viper.SetDefault("debug_cycle", false) //set cycle debug mode (prevent changes in cycle repository)

	path, err := osext.ExecutableFolder()
	if err != nil {
		path = "."
	}
	//fmt.Println("Path ", path)
	viper.AddConfigPath(path)
	viper.SetConfigName("config")
	return viper.ReadInConfig()
}
