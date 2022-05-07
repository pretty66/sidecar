package confer

import (
	"fmt"
	"github.com/openmsp/sidecar/utils"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/go-chassis/go-archaius"
)

var (
	RemoteConfigEnable bool
	RemoteConfigAddr   string
)

func init() {
	RemoteConfigEnable = os.Getenv("MSP_CONFIG_CENTER_ENABLE") == "true"
	RemoteConfigAddr = os.Getenv("MSP_CONFIG_CENTER_ADDR")
}

// configuring a priority
// 0: remote source - pull remote config server data into local
// 1: Memory source - after init, you can set key value in runtime.
// 2: Command Line source - read the command lines arguments, while starting the process.
// 3: Environment Variable source - read configuration in Environment variable.
// 4: Files source - read files content and convert it into key values based on the FileHandler you define
func InitConfig(serviceType, confFileURI, version string) (*Confer, error) {
	if _confer != nil {
		return _confer, nil
	}
	if RemoteConfigEnable {
		remoteConf, err := GetRemoteConfig(serviceType, RemoteConfigAddr, version)
		if err != nil {
			panic(err)
		}
		for k, v := range remoteConf {
			err := os.Setenv(k, v)
			if err != nil {
				panic(err)
			}
		}
	}
	err := archaius.Init(
		archaius.WithENVSource(),
		archaius.WithRequiredFiles([]string{confFileURI}),
	)
	if err != nil {
		panic(err)
	}
	var conf Confer
	err = archaius.UnmarshalConfig(&conf.Opts)
	if err != nil {
		panic(err)
	}
	conf.Opts.ServiceType = serviceType
	if !utils.InArray(conf.Opts.TrafficInflow.BindProtocolType, []string{HTTP, HTTPS, MTLS}) {
		conf.Opts.TrafficInflow.BindProtocolType = HTTP
	}
	_, _, err = net.SplitHostPort(conf.Opts.TrafficInflow.TargetAddress)
	if err != nil {
		log.Printf("TrafficInflow.TargetAddress error: %s use default: 127.0.0.1:80", conf.Opts.TrafficInflow.TargetAddress)
		conf.Opts.TrafficInflow.TargetAddress = "127.0.0.1:80"
	}

	// Disable master-slave percentage of redis read-write separation
	conf.Opts.RedisClusterConf.SlaveOperateRate = 0
	if conf.Opts.RedisClusterConf.SwitchProxyCycle <= 0 {
		conf.Opts.RedisClusterConf.SwitchProxyCycle = 3600
	}

	// -----   redis cluster
	// Guarantee the lower limit of the heartbeat of the cluster: the minimum frequency is 5 seconds
	if conf.Opts.RedisClusterConf.ClusterUpdateHeartbeat < 5 {
		conf.Opts.RedisClusterConf.ClusterUpdateHeartbeat = 5
	}
	// ------  redis end

	// Check for Debug parameters: If Debug monitoring is enabled
	if conf.Opts.DebugConf.Enable {
		// If the listening address is an empty string in the scenario where Enable is enabled, it indicates that the configuration is incorrect
		if len(conf.Opts.DebugConf.PprofURI) == 0 {
			conf.Opts.DebugConf.PprofURI = "0.0.0.0:16060"
		}
	}
	// -----  Memory ------
	// If the middleware has enabled the cache function
	if conf.Opts.MemoryCacheConf.Enable {
		// Cache related configuration parameter security verification: set the minimum number of items in the cache
		if conf.Opts.MemoryCacheConf.MaxItemsCount < 1024 {
			conf.Opts.MemoryCacheConf.MaxItemsCount = 1024
		}
	}

	// memory cache expires by default ms lower limit check
	if conf.Opts.MemoryCacheConf.DefaultExpiration <= 0 {
		conf.Opts.MemoryCacheConf.DefaultExpiration = 0
	}

	// memory cache expiry kv automatic cleaning cycle minimum cycle limit (minimum 10s)
	if conf.Opts.MemoryCacheConf.CleanupInterval <= 10 {
		conf.Opts.MemoryCacheConf.CleanupInterval = 10
	}
	// ----- Memory end ------

	// ---- log ---- // 192.168.2.80:9822
	if conf.Opts.Log.OutPut == "redis" {
		host, port, err := net.SplitHostPort(conf.Opts.Log.RedisHost)
		if err != nil {
			panic(fmt.Errorf("redishost parse err: %s, %v", conf.Opts.Log.RedisHost, err))
		}
		conf.Opts.Log.RedisHost = host
		conf.Opts.Log.RedisPort, err = strconv.Atoi(port)
		if err != nil {
			panic(err)
		}
	}

	// ---- log end

	// ----- rate limiter
	conf.replaceByEnv(&conf.Opts.RateLimiter.RedisClusterNodes)
	if conf.Opts.RateLimiter.MaxConnAge <= 0 {
		conf.Opts.RateLimiter.MaxConnAge = 1
	}
	if conf.Opts.RateLimiter.IdleCheckFrequency <= 0 {
		conf.Opts.RateLimiter.IdleCheckFrequency = 1
	}
	// ----- rate limiter end

	if conf.Opts.Discovery.HeartBeatToMsp <= 0 {
		conf.Opts.Discovery.HeartBeatToMsp = 10
	}
	if !utils.InArray(conf.Opts.Discovery.Scheme, []string{MTLS, HTTP}) {
		conf.Opts.Discovery.Scheme = HTTP
	}
	// ----- discovery end

	if len(conf.Opts.MiscroServiceInfo.Hostname) == 0 {
		hostTemp, err := os.Hostname()
		if err == nil {
			conf.Opts.MiscroServiceInfo.Hostname = hostTemp
		}
	}

	conf.Opts.MiscroServiceInfo.MetaData["runtime"] = conf.Opts.MiscroServiceInfo.Env
	conf.Opts.MiscroServiceInfo.MetaData["service_name"] = conf.Opts.MiscroServiceInfo.ServiceName

	// confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.EndPointAddress)
	if len(conf.Opts.MiscroServiceInfo.PodIP) == 0 {
		panic("The service access address is incorrectly configured!")
	}
	err = conf.handleSidecarEndpoint()
	if err != nil {
		return nil, err
	}
	if conf.Opts.MiscroServiceInfo.HeartBeat <= 0 {
		conf.Opts.MiscroServiceInfo.HeartBeat = 30
	}

	// bbr
	if len(conf.Opts.BBR.RemoteResourceURL) == 0 {
		conf.Opts.BBR.Enable = false
	}
	// influxdb
	if conf.Opts.InfluxDBConf.Enable {
		host, port, err := net.SplitHostPort(conf.Opts.InfluxDBConf.Address)
		if err != nil {
			log.Println("influxdb host port is wrong err: ", err)
		} else {
			conf.Opts.InfluxDBConf.Address = host
			portInt, _ := strconv.Atoi(port)
			conf.Opts.InfluxDBConf.Port = portInt
		}

		if flushSize := conf.Opts.InfluxDBConf.FlushSize; flushSize == 0 {
			conf.Opts.InfluxDBConf.FlushSize = 20
		}
		if flushTime := conf.Opts.InfluxDBConf.FlushTime; flushTime == 0 {
			conf.Opts.InfluxDBConf.FlushTime = 15
		}
		conf.Opts.InfluxDBConf.Precision = strings.ToLower(conf.Opts.InfluxDBConf.Precision)
		switch conf.Opts.InfluxDBConf.Precision {
		case "ns", "us", "ms", "s", "m", "h":
		default:
			conf.Opts.InfluxDBConf.Precision = "ms"
		}
	}
	// end influxdb

	conf.Opts.Heartbeat.API = "http://" + conf.Opts.TrafficInflow.TargetAddress + conf.Opts.Heartbeat.API
	// end healthcheck

	// msp_event
	// ....
	// end msp_event

	// ca start
	if conf.Opts.CaConfig.SecurityMode < 1 || conf.Opts.CaConfig.SecurityMode > 2 {
		conf.Opts.CaConfig.SecurityMode = 1
	}
	conf.Opts.MiscroServiceInfo.MetaData["cert_sn"] = ""
	if len(conf.Opts.CaConfig.AddressOCSP) == 0 {
		conf.Opts.CaConfig.AddressOCSP = conf.Opts.CaConfig.Address
	}
	// end ca

	_confer = &conf
	return _confer, nil
}
