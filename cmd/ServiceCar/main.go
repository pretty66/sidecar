package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/pkg/redisproxy"
	"github.com/openmsp/sidecar/sc"
	"github.com/openmsp/sidecar/sc/sidecar"
	"github.com/openmsp/sidecar/utils"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"syscall"
	"time"

	"github.com/openmsp/cilog"
	"github.com/urfave/cli"
	_ "go.uber.org/automaxprocs"
)

// BuildDate: Binary file compilation time
// BuildVersion: Binary compiled GIT version
// BuildTag: Binary compiled GIT Tag
var (
	BuildDate    string
	BuildVersion string
	BuildTag     string
)

func main() {
	app := cli.NewApp()

	app.Name = os.Getenv("MSP_SERVICE_TYPE")
	if app.Name == "" {
		app.Name = confer.SERVICE_TYPE_SIDECAR
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:     "config, c",
			Usage:    "init config file",
			EnvVar:   "MSP_SC_CONFIG",
			Value:    "config/sc_config.yaml",
			Required: false,
		},
	}
	app.Before = func(ctx *cli.Context) error {
		showBanner()
		log.SetFlags(log.Lshortfile | log.LstdFlags)
		return nil
	}

	app.Action = func(ctx *cli.Context) error {
		os.Args = os.Args[:1]
		defer func() {
			if e := recover(); e != nil {
				cilog.LogErrorf(cilog.LogNameSidecar, "panic: %v", e)
			}
		}()
		configFile := ctx.String("config")
		if len(configFile) != 0 && !utils.IsFileExist(configFile) {
			return errors.New(configFile + ": No such file or directory")
		}

		conf, err := confer.InitConfig(app.Name, configFile, BuildTag)
		if err != nil {
			panic(err)
		}

		if conf.Opts.DebugConf.Enable {
			utils.GoWithRecover(func() {
				startDebug(conf.Opts.DebugConf.PprofURI)
			}, nil)
		}
		c, cancel := context.WithCancel(context.Background())
		offlineCtx, offlineCancel := context.WithCancel(c)
		serviceCar := sc.NewSc(c, offlineCtx, conf)
		initFunc := []func() error{
			serviceCar.LoadBasics,
		}
		var tp *sidecar.TrafficProxy
		switch conf.Opts.ServiceType {
		case confer.SERVICE_TYPE_SIDECAR:
			tp = &sidecar.TrafficProxy{SC: serviceCar}
			initFunc = append(initFunc,
				tp.LoadConfigByUniqueID,
				tp.LoadMetrics,
				tp.LoadTrace,
				tp.LoadCaVerify,
				tp.LoadBBR,
				tp.LoadRouterRule,
				tp.LoadACL,
				tp.StartTrafficProxy,
				tp.RegisterToDiscovery,
				serviceCar.StartRemoteConfigFetcher,
				tp.StartRouterConfigFetcher,
			)
		case confer.SERVICE_TYPE_GATEWAY:
		default:
			panic("run service type error:" + conf.Opts.ServiceType)
		}
		for k := range initFunc {
			err := initFunc[k]()
			if err != nil {
				fnName := runtime.FuncForPC(reflect.ValueOf(initFunc[k]).Pointer()).Name()
				log.Println("Error function：", k, fnName)
				panic(err)
			}
		}
		if tp != nil {
			utils.GoWithRecover(func() {
				redisproxy.Start(c, serviceCar.Exchanger)
			}, nil)
		}

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR1)
		isOffline := false
		var t1 time.Time
		for sig := range sigs {
			switch sig {
			case syscall.SIGUSR1:
				if isOffline {
					continue
				}
				offlineCancel()
				_ = serviceCar.Discovery.Close()
				isOffline = true
				t1 = time.Now()
				cilog.LogWarnf(cilog.LogNameSidecar, "signal：%s, offline stop accepting new traffic！", sig)
			case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
				if !isOffline {
					_ = serviceCar.Discovery.Close()
					t1 = time.Now()
				}
				cancel()
				if serviceCar.Confer.Opts.CaConfig.Enable {
					_ = serviceCar.Exchanger.RevokeItSelf()
				}
				serviceCar.ShutdownWg.Wait()
				cilog.LogWarnf(cilog.LogNameSidecar, "signal：%s, sidecar process exits，Smooth wait for request processing to complete, wait time：%s", sig.String(), time.Now().Sub(t1))
				os.Exit(0)
			case syscall.SIGHUP:
				log.Println("+++++++++++++++++++++++++++++")
			}
		}
		defer func() {
			cancel()
			offlineCancel()
			cilog.LogWarnf(cilog.LogNameSidecar, "Abnormal exit, no exit signal is monitored, time consuming：", time.Now().Sub(t1))
		}()
		return nil
	}
	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}

func startDebug(pprofURI string) {
	log.Printf("ServiceCar pprof listen on: %s\n", pprofURI)
	err := http.ListenAndServe(pprofURI, nil)
	if err != nil {
		return
	}
}

func showBanner() {
	bannerData := `  +===============================+
  |    /$$$$$$         /$$$$$$    |
  |   /$$__  $$       /$$__  $$   |
  |  | $$  \__/      | $$  \__/   |
  |  |  $$$$$$       | $$         |
  |   \____  $$      | $$         |
  |   /$$  \ $$      | $$    $$   |
  |  |  $$$$$$/      |  $$$$$$/   |
  |   \______/        \______/    |				  
  +===============================+`
	fmt.Println(bannerData)
	fmt.Println("Start sidecar MAXPROCS set to：", runtime.GOMAXPROCS(0))
	fmt.Println("Build Version: ", BuildVersion, "  Date: ", BuildDate, "  Tag: ", BuildTag)
}
