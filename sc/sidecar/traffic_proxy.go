package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/openmsp/sidecar/pkg/acl"
	"github.com/openmsp/sidecar/pkg/ca"
	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/pkg/fuse"
	"github.com/openmsp/sidecar/pkg/metrics"
	"github.com/openmsp/sidecar/pkg/ratelimit"
	"github.com/openmsp/sidecar/pkg/ratelimit/bbr"
	"github.com/openmsp/sidecar/pkg/router"
	"github.com/openmsp/sidecar/pkg/trace"
	"github.com/openmsp/sidecar/sc"
	"github.com/openmsp/sidecar/utils"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/openmsp/kit/rediscluster"

	"github.com/openmsp/cilog"
	"github.com/openmsp/sesdk"
	"github.com/openmsp/sesdk/discovery"
	"github.com/paulbellamy/ratecounter"
	network "knative.dev/networking/pkg"
)

type TrafficProxy struct {
	*sc.SC
	TrafficOutFuse        *fuse.CircuitBreaker
	TrafficInRateLimiter  *ratelimit.RateLimiter
	TrafficInRateCounter  *ratecounter.RateCounter
	TrafficOutRateCounter *ratecounter.RateCounter
	BBRLimiter            ratelimit.Limiter
	CaVerify              *ca.CertStatus
	Metrics               *metrics.Metrics
	Router                *router.Rule
	State                 *network.RequestStats
}

func (tp *TrafficProxy) StartTrafficProxy() (err error) {
	tp.InflowProtocol.Store(tp.Confer.Opts.TrafficInflow.BindProtocolType)

	tp.TrafficOutFuse = fuse.GetCircuitBreaker()

	tp.TrafficInRateLimiter = ratelimit.NewRateLimiter(
		tp.Confer.Opts.MiscroServiceInfo.UniqueID,
		tp.Confer.Opts.MiscroServiceInfo.ServiceName,
		tp.Confer.Opts.MiscroServiceInfo.Hostname,
		tp.Confer.Opts.RateLimiter,
		tp.RedisCluster,
		tp.SC)

	tp.TrafficInRateCounter = ratecounter.NewRateCounter(time.Second * 60)
	tp.TrafficOutRateCounter = ratecounter.NewRateCounter(time.Second * 60)

	if err = startHTTPReverseProxy(tp); err != nil {
		return err
	}

	if runtime.GOOS == "linux" {
		tp.startAttributesHeartbeat()
	}
	return nil
}

func (tp *TrafficProxy) startAttributesHeartbeat() {
	utils.GoWithRecover(func() {
		tp.microservicesAttributesHeartbeat()
	}, func(r interface{}) {
		time.Sleep(time.Second * 5)
		tp.startAttributesHeartbeat()
	})
}

func (tp *TrafficProxy) LoadConfigByUniqueID() error {
	defer func() {
		log.Println("access scheme：", tp.Confer.Opts.TrafficInflow.BindProtocolType)
	}()

	if !tp.Confer.Opts.CaConfig.Enable {
		return nil
	}
	mode, err := rediscluster.Bytes(tp.RedisCluster.Do("HGET", fmt.Sprintf("msp:service_rule:%s", tp.Confer.Opts.MiscroServiceInfo.UniqueID), "mode"))

	if err == rediscluster.ErrNil || len(mode) == 0 {
		return nil
	}
	if err != nil {
		return err
	}
	newMode := string(mode)
	if newMode == tp.Confer.Opts.TrafficInflow.BindProtocolType {
		return nil
	}
	scheme := confer.HTTP
	if utils.InArray(newMode, []string{confer.HTTPS, confer.MTLS}) {
		scheme = confer.HTTPS
	}
	port := utils.GetPort(tp.Confer.Opts.TrafficInflow.BindAddress)
	tp.Confer.Opts.TrafficInflow.BindProtocolType = newMode
	tp.InflowProtocol.Store(newMode)
	tp.Confer.Opts.MiscroServiceInfo.MetaData["mode"] = newMode
	tp.Confer.Opts.MiscroServiceInfo.EndPointAddress = fmt.Sprintf("%s://%s:%s", scheme, tp.Confer.Opts.MiscroServiceInfo.PodIP, port)
	return nil
}

func (tp *TrafficProxy) LoadBBR() (err error) {
	if !tp.Confer.Opts.BBR.Enable {
		return
	}
	err = os.Setenv("REMOTE_RESOURCE_URL", tp.Confer.Opts.BBR.RemoteResourceURL)
	if err != nil {
		return err
	}

	group, err := bbr.NewGroup(&bbr.Config{
		Window:       time.Second * 10,
		WinBucket:    100,
		CPUThreshold: 800,
	})
	if err != nil {
		return err
	}
	tp.BBRLimiter = group.Get("ServiceCar")
	return nil
}

func (tp *TrafficProxy) RegisterToDiscovery() (err error) {
	tp.Instance = &sesdk.Instance{
		Zone:     tp.Confer.Opts.MiscroServiceInfo.Zone,
		Env:      tp.Confer.Opts.Discovery.Env,
		AppID:    tp.Confer.Opts.MiscroServiceInfo.UniqueID,
		Region:   tp.Confer.Opts.MiscroServiceInfo.Namespace,
		Addrs:    []string{tp.Confer.Opts.MiscroServiceInfo.EndPointAddress},
		LastTs:   time.Now().Unix(),
		Metadata: tp.Confer.Opts.MiscroServiceInfo.MetaData,
		Hostname: tp.Confer.Opts.MiscroServiceInfo.Hostname,
		Status:   discovery.InstanceStatusNotReceive,
		Version:  tp.Confer.Opts.MiscroServiceInfo.MetaData["version"],
	}

	if !tp.Confer.Opts.Heartbeat.Enable {
		tp.Instance.Status = discovery.InstanceStatusReceive
	}
	_, err = tp.Discovery.Register(context.TODO(), tp.Instance)
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "Failed to register the instance to the dynamic registry", err)
		return
	}

	if tp.Confer.Opts.Heartbeat.Enable {
		utils.GoWithRecover(func() {
			tp.heartbeat(tp.Instance)
		}, nil)
	}

	if tp.Confer.Opts.K8SHeartbeat.Enable {
		utils.GoWithRecover(func() {
			tp.k8sheartbeat()
		}, nil)
	}

	return nil
}

func (tp *TrafficProxy) LoadTrace() (err error) {
	c := tp.Confer.Opts.AutoTrace
	var conf trace.Config

	configByte, err := rediscluster.Bytes(tp.RedisCluster.Do("HGET", fmt.Sprintf("msp:service_rule:%s", tp.Confer.Opts.MiscroServiceInfo.UniqueID), "trace"))
	if err == rediscluster.ErrNil || err == redis.Nil || len(configByte) == 0 {
		conf.Enable = c.Enable
		conf.Type = c.Type
	} else {
		err = json.Unmarshal(configByte, &conf)
		if err != nil {
			cilog.LogErrorf(cilog.LogNameSidecar, "load config form redis parse err %s", err)
			return err
		}
	}
	channel := tp.RegisterConfigWatcher("trace", func(key, value []byte) bool {
		return string(key) == "trace"
	}, 1)

	tp.Trace, err = trace.NewTrace(
		c.SkyCollectorGrpcAddress,
		conf,
		channel,
	)
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "apmAgent error", err)
		return err
	}
	return nil
}

func (tp *TrafficProxy) LoadMetrics() error {
	if tp.Confer.Opts.InfluxDBConf.Enable {
		tp.Metrics = metrics.NewMetrics()
	}
	return nil
}

func (tp *TrafficProxy) LoadCaVerify() error {
	tp.CaVerify = ca.InitCaCertStatusVerify(tp.SC)
	return nil
}

func (tp *TrafficProxy) LoadRouterRule() error {
	tp.Router = router.InitRouterRedirect(tp.OfflineCtx, tp.Confer.Opts.RedisClusterConf.UseProxy)
	return nil
}

func (tp *TrafficProxy) StartRouterConfigFetcher() error {
	utils.GoWithRecover(func() {
		tp.Router.ListenConfig()
	}, func(r interface{}) {
		time.Sleep(time.Second * 3)
		tp.StartRouterConfigFetcher()
	})
	return nil
}

func (tp *TrafficProxy) LoadACL() error {
	acl.InitACL()
	return nil
}
