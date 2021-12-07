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

	"github.com/egorka-gh/pixlpark/evropochta"
	cycle "github.com/egorka-gh/pixlpark/photocycle"
	log "github.com/go-kit/kit/log"
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

	svcConfig := &service1.Config{
		Name:        "Evropochta",
		DisplayName: "Evropochta Service",
		Description: "Evropochta service for PhotoCycle",
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
	g, err := initEvropochta()
	if err != nil {
		return err
	}

	p.group = g
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
				return errors.New("evropochta: Get interrupt signal")
			case <-running:
				return nil
			}
		}, func(error) {
			close(running)
		})
	dLogger.Info("Evropochta started")
	dLogger.Info(p.group.Run())
	close(p.quit)
}

func (p *program) Stop(s service1.Service) error {
	// Stop should not block. Return with a few seconds.
	dLogger.Info("Evropochta Stopping!")
	//interrupt service
	close(p.interrupt)
	//waite service stops
	<-p.quit
	dLogger.Info("Evropochta stopped")
	return nil
}

func initEvropochta() (*group.Group, error) {
	//TODO check settings
	if viper.GetString("evropochta.user") == "" ||
		viper.GetString("evropochta.pass") == "" ||
		viper.GetString("evropochta.serviceNumber") == "" {
		return nil, errors.New("не заданы параметры входа в evropochta")
	}
	if viper.GetString("evropochta.address") == "" {
		return nil, errors.New("не задан host:port для локального сервера")
	}
	//TODO log startup params

	logger := initLoger(viper.GetString("folders.log"), "evropochta")

	evropochtaClient, err := evropochta.NewClient(
		viper.GetString("evropochta.baseURL"),
		viper.GetString("evropochta.user"),
		viper.GetString("evropochta.pass"),
		viper.GetString("evropochta.serviceNumber"),
		viper.GetString("evropochta.outFolder"),
		logger)
	if err != nil {
		return nil, err
	}

	//init proxy
	pcfg := evropochta.HandlerConfig{
		Client: evropochtaClient,
		Logger: logger,
	}

	server := &http.Server{
		Addr:         viper.GetString("evropochta.address"),
		Handler:      evropochta.NewHandler(&pcfg),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  15 * 60 * time.Second,
	}

	g := &group.Group{}
	g.Add(func() error {
		//logger.Log("transport", "debug/HTTP", "addr", debugAddr)
		dLogger.Info(fmt.Sprintf("Starting proxy at %s.", server.Addr))
		//dLogger.Info(fmt.Sprintf("Debug endpoint at %s/debug/pprof/.", server.Addr))
		return server.ListenAndServe()
	}, func(error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})

	return g, nil
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
	viper.SetDefault("folders.log", ".\\log")     //Log folder
	viper.SetDefault("evropochta.address", ":81") //localhost
	viper.SetDefault("evropochta.baseURL", "https://api.eurotorg.by:10352/Json")
	viper.SetDefault("evropochta.user", "")
	viper.SetDefault("evropochta.pass", "")
	viper.SetDefault("evropochta.serviceNumber", "")
	viper.SetDefault("evropochta.outFolder", ".\\evropochta")

	path, err := osext.ExecutableFolder()
	if err != nil {
		path = "."
	}
	//fmt.Println("Path ", path)
	viper.AddConfigPath(path)
	viper.SetConfigName("config")
	return viper.ReadInConfig()
}
