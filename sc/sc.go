package sc

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/openmsp/sidecar/pkg/autoredis"
	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/pkg/event"
	"github.com/openmsp/sidecar/pkg/remoteconfer"
	"github.com/openmsp/sidecar/pkg/trace"
	"github.com/openmsp/sidecar/utils/influxdb"
	cache "github.com/openmsp/sidecar/utils/memorycacher"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/openmsp/sesdk"

	"github.com/openmsp/cilog"
	"github.com/openmsp/cilog/redis_hook"
	cv2 "github.com/openmsp/cilog/v2"
	client "github.com/openmsp/kit/influxdata/influxdb1-client/v2"
	"github.com/openmsp/sesdk/discovery"
	"github.com/sirupsen/logrus"
	"github.com/ztalab/ZACA/pkg/caclient"
	"github.com/ztalab/ZACA/pkg/spiffe"
	"go.uber.org/zap/zapcore"
)

type SC struct {
	Ctx                context.Context
	Confer             *confer.Confer
	RedisCluster       autoredis.AutoClient
	Cache              *cache.Cache
	Trace              *trace.Trace
	Metrics            *influxdb.Metrics
	Discovery          *discovery.Discovery
	Instance           *sesdk.Instance
	DiscoveryTLSConfig *tls.Config
	InfluxDBUDPClient  *influxdb.InfluxDBUDPClient
	InfluxDBHttpClient *influxdb.InfluxDBHttpClient

	Exchanger  *caclient.Exchanger
	ShutdownWg sync.WaitGroup

	configFetcher  *remoteconfer.RemoteConfigFetcher
	InflowProtocol atomic.Value
	OfflineCtx     context.Context
	sync.RWMutex
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func (sc *SC) LoadBasics() (err error) {
	configLogData := &cilog.ConfigLogData{
		OutPut: sc.Confer.Opts.Log.OutPut,
		Debug:  sc.Confer.Opts.Log.Debug,
		Key:    sc.Confer.Opts.Log.LogKey,
		Redis: struct {
			Host string
			Port int
		}{Host: sc.Confer.Opts.Log.RedisHost, Port: sc.Confer.Opts.Log.RedisPort},
	}
	configAppData := &cilog.ConfigAppData{
		AppName:    sc.Confer.Opts.MiscroServiceInfo.ServiceName,
		AppID:      sc.Confer.Opts.MiscroServiceInfo.AppID,
		AppVersion: sc.Confer.Opts.MiscroServiceInfo.MetaData["version"],
	}
	cilog.ConfigLogInit(configLogData, configAppData)
	cv2.GlobalConfig(cv2.Conf{
		Caller:     false,
		Debug:      false,
		Level:      zapcore.InfoLevel,
		StackLevel: zapcore.DPanicLevel,
		AppInfo:    configAppData,
		HookConfig: &redis_hook.HookConfig{
			Key:  sc.Confer.Opts.Log.LogKey,
			Host: sc.Confer.Opts.Log.RedisHost,
			Port: sc.Confer.Opts.Log.RedisPort,
		},
	})

	logrus.SetLevel(logrus.WarnLevel)

	if sc.Confer.Opts.CaConfig.Enable {
		err = sc.StartTLSConn()
		if err != nil {
			return err
		}
		if sc.Confer.Opts.Discovery.Scheme == confer.MTLS {
			sc.DiscoveryTLSConfig, err = sc.NewSidecarMTLSClient(discovery.ServerName)
			if err != nil {
				return err
			}
		}
	}

	conf := &discovery.Config{
		Nodes:     strings.Split(sc.Confer.Opts.Discovery.Address, ","),
		Zone:      sc.Confer.Opts.MiscroServiceInfo.Zone,
		Env:       sc.Confer.Opts.Discovery.Env,
		Region:    sc.Confer.Opts.MiscroServiceInfo.Region,
		Host:      sc.Confer.Opts.MiscroServiceInfo.Hostname,
		RenewGap:  time.Second * time.Duration(sc.Confer.Opts.MiscroServiceInfo.HeartBeat),
		TLSConfig: sc.DiscoveryTLSConfig,
	}

	sc.Discovery, err = discovery.New(conf)
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "discovery new", err)
		return
	}

	if sc.Confer.Opts.RedisClusterConf.Enable {
		sc.RedisCluster, err = autoredis.NewAutoRedisClient(sc.Ctx, sc.Discovery, confer.Global().Opts.RedisClusterConf, sc.Exchanger)
		if err != nil {
			return
		}
	}

	sc.Confer.Opts.MemoryCacheConf.Enable = true

	if sc.Confer.Opts.MemoryCacheConf.Enable {
		sc.Cache = cache.New(
			time.Millisecond*time.Duration(confer.Global().Opts.MemoryCacheConf.DefaultExpiration),
			time.Second*time.Duration(confer.Global().Opts.MemoryCacheConf.CleanupInterval),
			confer.Global().Opts.MemoryCacheConf.MaxItemsCount,
		)
	}
	sc.configFetcher = remoteconfer.NewConfigFetcher(sc.Ctx, sc.RedisCluster, sc.Confer.Opts.MiscroServiceInfo.UniqueID, sc.Confer.Opts.RemoteConfig.Cycle,
		remoteconfer.FetcherConfig{
			IsWatchEnabled: sc.RedisCluster.IsTypeProxy(),
			HostName:       sc.Confer.Opts.MiscroServiceInfo.Hostname,
			StreamKey:      "watch_config:msp:service_rule",
		})
	event.InitEventClient(sc.RedisCluster)
	if sc.Confer.Opts.InfluxDBConf.Enable {
		httpClient, err := client.NewHTTPClient(client.HTTPConfig{
			Addr:                fmt.Sprintf("http://%s:%d", sc.Confer.Opts.InfluxDBConf.Address, sc.Confer.Opts.InfluxDBConf.Port),
			Username:            sc.Confer.Opts.InfluxDBConf.UserName,
			Password:            sc.Confer.Opts.InfluxDBConf.Password,
			MaxIdleConns:        sc.Confer.Opts.InfluxDBConf.MaxIdleConns,
			MaxIdleConnsPerHost: sc.Confer.Opts.InfluxDBConf.MaxIdleConnsPerHost,
			IdleConnTimeout:     90 * time.Second,
			Timeout:             10 * time.Second,
		})
		if err != nil {
			cilog.LogErrorw(cilog.LogNameSidecar, "init influx http client error", err)
			return err
		}
		bpc := client.BatchPointsConfig{
			Database:  sc.Confer.Opts.InfluxDBConf.Database,
			Precision: sc.Confer.Opts.InfluxDBConf.Precision,
		}

		sc.Metrics = influxdb.NewMetrics(
			sc.Ctx,
			&influxdb.HTTPClient{
				Client: httpClient,
				BatchPointsConfig: client.BatchPointsConfig{
					Precision: sc.Confer.Opts.InfluxDBConf.Precision,
					Database:  sc.Confer.Opts.InfluxDBConf.Database,
				},
			},
			sc.Confer.Opts.InfluxDBConf,
		)
		sc.InfluxDBHttpClient = &influxdb.InfluxDBHttpClient{Client: httpClient, BatchPointsConfig: bpc}
		if _, _, err := sc.InfluxDBHttpClient.Client.Ping(10 * time.Second); err != nil {
			cilog.LogErrorw(cilog.LogNameSidecar, "ping influxdb failed", err)
		}
	} else {
		sc.Metrics = influxdb.NewMetrics(sc.Ctx, nil, sc.Confer.Opts.InfluxDBConf)
	}
	return nil
}

func (sc *SC) StartTLSConn() error {
	if !sc.Confer.Opts.CaConfig.Enable {
		return nil
	}
	retry := 0
	for {
		if sc.Exchanger != nil {
			break
		}
		err := sc.newMTLS()
		if err != nil {
			retry++
			sleepSecond := retry * 5
			if sleepSecond > 120 {
				sleepSecond = 120
			}
			cilog.LogWarnf(cilog.LogNameSidecar, "init ca fail：%s, Retry after %s seconds", err.Error(), sleepSecond)
			time.Sleep(time.Second * time.Duration(sleepSecond))
			continue
		}
		break
	}
	return nil
}

func (sc *SC) newMTLS() (err error) {
	if !sc.Confer.Opts.CaConfig.Enable {
		return nil
	}
	l, _ := cv2.NewZapLogger(&cv2.Conf{
		Level: 2,
	})
	c := caclient.NewCAI(
		caclient.WithCAServer(caclient.RoleSidecar, sc.Confer.Opts.CaConfig.Address),
		caclient.WithAuthKey(sc.Confer.Opts.CaConfig.AuthKey),
		caclient.WithLogger(l),
		caclient.WithOcspAddr(sc.Confer.Opts.CaConfig.AddressOCSP),
	)
	exchanger, err := c.NewExchanger(&spiffe.IDGIdentity{
		SiteID:    sc.Confer.Opts.MiscroServiceInfo.Region,
		ClusterID: sc.Confer.Opts.MiscroServiceInfo.Zone,
		UniqueID:  sc.Confer.Opts.MiscroServiceInfo.UniqueID,
	})
	if err != nil {
		return err
	}
	_, err = exchanger.Transport.GetCertificate()
	if err != nil {
		return err
	}

	go exchanger.RotateController().Run()

	sc.Confer.Opts.MiscroServiceInfo.MetaData["cert_sn"] = ""
	if sc.Confer.Opts.CaConfig.Enable && exchanger != nil {
		cert, err := exchanger.Transport.GetCertificate()
		if err != nil {
			cilog.LogErrorw(cilog.LogNameSidecar, "The certificate ID is incorrect. Procedure", err)
		} else {
			sc.Confer.Opts.MiscroServiceInfo.MetaData["cert_sn"] = cert.Leaf.SerialNumber.String()
		}
	}
	sc.Exchanger = exchanger
	return nil
}

func (sc *SC) NewSidecarMTLSClient(host string) (*tls.Config, error) {
	cfger, err := sc.Exchanger.ClientTLSConfig(host)
	if err != nil {
		return nil, err
	}
	return cfger.TLSConfig(), nil
}

func (sc *SC) NewSidecarTLSServer(tlsType string) (*tls.Config, error) {
	var tlsCfg *caclient.TLSGenerator
	var err error

	switch tlsType {
	case confer.MTLS:
		tlsCfg, err = sc.Exchanger.ServerTLSConfig()
	case confer.HTTPS:
		tlsCfg, err = sc.Exchanger.ServerHTTPSConfig()
	default:
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	tlsCfg.BindExtraValidator(func(identity *spiffe.IDGIdentity) error {
		return nil
	})

	return tlsCfg.TLSConfig(), nil
}

func ShutdownEchoServer(ctx context.Context, server *http.Server, done func(err error)) {
	var err error
	select {
	case <-ctx.Done():
		ctxs, _ := context.WithTimeout(context.Background(), time.Second*10)
		err = server.Shutdown(ctxs)
	}
	done(err)
}

func (sc *SC) RegisterConfigWatcher(key string, c remoteconfer.ConfigAcceptor, buffer int) <-chan []byte {
	return sc.configFetcher.AddWatcher(key, c, buffer)
}

func (sc *SC) StartRemoteConfigFetcher() (err error) {
	go sc.configFetcher.Start()
	return nil
}
