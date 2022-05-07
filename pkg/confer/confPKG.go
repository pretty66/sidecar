package confer

import (
	"errors"
	"fmt"
	"github.com/openmsp/sidecar/utils"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	HTTP  = "http"
	HTTPS = "https"
	MTLS  = "mtls"
)

const (
	// Traffic management
	SERVICE_TYPE_SIDECAR = "sidecar"
	// As a cluster gateway
	SERVICE_TYPE_GATEWAY = "gateway"
)

var _confer *Confer

func GetNewConfer(serviceType, confFileURI string) (confer *Confer, err error) {
	if _confer != nil {
		return _confer, nil
	}
	confer = &Confer{}
	confer.Opts, err = parseYamlFromFile(confFileURI)
	confer.Opts.ServiceType = serviceType

	confer.replaceByEnv(&confer.Opts.TrafficInflow.BindProtocolType)
	if !utils.InArray(confer.Opts.TrafficInflow.BindProtocolType, []string{HTTP, HTTPS, MTLS}) {
		confer.Opts.TrafficInflow.BindProtocolType = HTTP
	}

	if addr := os.Getenv(confer.Opts.TrafficInflow.TargetAddress); len(addr) > 0 {
		if strings.Index(addr, "http") != -1 {
			return nil, errors.New("TargetAddress error => host:port, example：127.0.0.1:80")
		}
		confer.Opts.TrafficInflow.TargetAddress = strings.Trim(addr, "/ ")
	} else {
		_, _, err := net.SplitHostPort(confer.Opts.TrafficInflow.TargetAddress)
		if err != nil {
			confer.Opts.TrafficInflow.TargetAddress = "127.0.0.1:80"
		}
	}

	// Disable master-slave percentage of redis read-write separation
	confer.Opts.RedisClusterConf.SlaveOperateRate = 0
	if confer.Opts.RedisClusterConf.SwitchProxyCycle <= 0 {
		confer.Opts.RedisClusterConf.SwitchProxyCycle = 3600
	}

	// -----   redis cluster
	// Guarantee the lower limit of the heartbeat of the cluster: the minimum frequency is 5 seconds
	if confer.Opts.RedisClusterConf.ClusterUpdateHeartbeat < 5 {
		confer.Opts.RedisClusterConf.ClusterUpdateHeartbeat = 5
	}
	confer.replaceByEnv(&confer.Opts.RedisClusterConf.StartNodes)
	// ------  redis end

	// Check for Debug parameters: If Debug monitoring is enabled
	if confer.Opts.DebugConf.Enable {
		// If the listening address is an empty string in the scenario where Enable is enabled, it indicates that the configuration is incorrect
		if len(confer.Opts.DebugConf.PprofURI) == 0 {
			err = errors.New("when the Enable parameter in Debug is True, the pprof_uri parameter cannot be empty, and a meaningful pprof listening address must be filled in")
			return
		}
	}
	// -----  Memory ------
	// If the middleware has enabled the cache function
	if confer.Opts.MemoryCacheConf.Enable {
		// Cache related configuration parameter security verification: set the minimum number of items in the cache
		if confer.Opts.MemoryCacheConf.MaxItemsCount < 1024 {
			confer.Opts.MemoryCacheConf.MaxItemsCount = 1024
		}
	}

	// memory cache expires by default ms lower limit check
	if confer.Opts.MemoryCacheConf.DefaultExpiration <= 0 {
		confer.Opts.MemoryCacheConf.DefaultExpiration = 0
	}

	// memory cache expiry kv automatic cleaning cycle minimum cycle limit (minimum 10s)
	if confer.Opts.MemoryCacheConf.CleanupInterval <= 10 {
		confer.Opts.MemoryCacheConf.CleanupInterval = 10
	}
	// ----- Memory end ------

	// ---- log ---- // 192.168.2.80:9822
	if logHost := os.Getenv(confer.Opts.Log.RedisHost); len(logHost) > 0 {
		host, port, err := net.SplitHostPort(logHost)
		if err != nil {
			panic(err)
		}
		confer.Opts.Log.RedisHost = host
		confer.Opts.Log.RedisPort, err = strconv.Atoi(port)
		if err != nil {
			panic(err)
		}
	}
	// ---- log end

	// ----- rate limiter
	confer.replaceByEnv(&confer.Opts.RateLimiter.RedisClusterNodes)
	if confer.Opts.RateLimiter.MaxConnAge <= 0 {
		confer.Opts.RateLimiter.MaxConnAge = 1
	}
	if confer.Opts.RateLimiter.IdleCheckFrequency <= 0 {
		confer.Opts.RateLimiter.IdleCheckFrequency = 1
	}
	// ----- rate limiter end

	// ----- trace
	confer.replaceByEnv(&confer.Opts.Trace.SkyCollectorGrpcAddress)
	// ----- trace end
	// ----- auto trace
	confer.replaceByEnv(&confer.Opts.AutoTrace.SkyCollectorGrpcAddress)
	// ----- auto trace end

	// ----- discovery
	confer.replaceByEnv(&confer.Opts.Discovery.Address)
	// attribute reporting
	if confer.Opts.Discovery.HeartBeatToMsp <= 0 {
		confer.Opts.Discovery.HeartBeatToMsp = 10
	}
	// discovery Runtime environment
	confer.replaceByEnv(&confer.Opts.Discovery.Env)
	confer.replaceByEnv(&confer.Opts.Discovery.Scheme)
	if !utils.InArray(confer.Opts.Discovery.Scheme, []string{MTLS, HTTP}) {
		confer.Opts.Discovery.Scheme = HTTP
	}
	// ----- discovery end

	confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.Region)
	confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.Zone)
	confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.Env)
	confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.AppID)

	confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.Hostname)

	if len(confer.Opts.MiscroServiceInfo.Hostname) == 0 {
		hostTemp, err := os.Hostname()
		if err == nil {
			confer.Opts.MiscroServiceInfo.Hostname = hostTemp
		}
	}

	confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.UniqueID)
	confer.Opts.MiscroServiceInfo.MetaData = make(map[string]string)
	if wk, ok := confer.Opts.MiscroServiceInfo.MetaData["weight"]; ok {
		if weight := os.Getenv(wk); len(weight) > 0 {
			confer.Opts.MiscroServiceInfo.MetaData["weight"] = weight
		} else {
			confer.Opts.MiscroServiceInfo.MetaData["weight"] = "10"
		}
	} else {
		confer.Opts.MiscroServiceInfo.MetaData["weight"] = "10"
	}
	if vs, ok := confer.Opts.MiscroServiceInfo.MetaData["version"]; ok {
		if version := os.Getenv(vs); len(version) > 0 {
			confer.Opts.MiscroServiceInfo.MetaData["version"] = version
		}
	}
	confer.Opts.MiscroServiceInfo.MetaData["runtime"] = confer.Opts.MiscroServiceInfo.Env
	confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.ServiceName)
	confer.Opts.MiscroServiceInfo.MetaData["service_name"] = confer.Opts.MiscroServiceInfo.ServiceName
	if si, ok := confer.Opts.MiscroServiceInfo.MetaData["service_image"]; ok {
		if si := os.Getenv(si); len(si) > 0 {
			confer.Opts.MiscroServiceInfo.MetaData["service_image"] = si
		}
	}
	if gu, ok := confer.Opts.MiscroServiceInfo.MetaData["service_gateway_addr"]; ok {
		if gu := os.Getenv(gu); len(gu) > 0 {
			confer.Opts.MiscroServiceInfo.MetaData["service_gateway_addr"] = gu
		} else {
			delete(confer.Opts.MiscroServiceInfo.MetaData, "service_gateway_addr")
		}
	}

	// confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.EndPointAddress)
	confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.NodeIP)
	confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.PodIP)
	if confer.Opts.MiscroServiceInfo.PodIP == "" {
		return nil, errors.New("The service access address is incorrectly configured！")
	}
	err = confer.handleSidecarEndpoint()
	if err != nil {
		return nil, err
	}
	if confer.Opts.MiscroServiceInfo.HeartBeat <= 0 {
		confer.Opts.MiscroServiceInfo.HeartBeat = 30
	}
	confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.ServiceMemRequests)
	confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.ServiceMemLimits)
	confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.ServiceCPURequests)
	confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.ServiceCPULimits)
	confer.replaceByEnv(&confer.Opts.MiscroServiceInfo.SidecarMEMLimits)

	// bbr
	if confer.Opts.BBR.Enable {
		if remoteResourceURL := os.Getenv(confer.Opts.BBR.RemoteResourceURL); len(remoteResourceURL) > 0 {
			confer.Opts.BBR.RemoteResourceURL = remoteResourceURL
		}
		if len(confer.Opts.BBR.RemoteResourceURL) == 0 {
			confer.Opts.BBR.Enable = false
		}
	}
	// influxdb
	if confer.Opts.InfluxDBConf.Enable {
		confer.replaceByEnv(&confer.Opts.InfluxDBConf.Address)

		host, port, err := net.SplitHostPort(confer.Opts.InfluxDBConf.Address)
		if err != nil {
			log.Println("influxdb host port is wrong err: ", err)
		} else {
			confer.Opts.InfluxDBConf.Address = host
			portInt, _ := strconv.Atoi(port)
			confer.Opts.InfluxDBConf.Port = portInt
		}

		confer.replaceByEnv(&confer.Opts.InfluxDBConf.UserName)
		confer.replaceByEnv(&confer.Opts.InfluxDBConf.Password)
		confer.replaceByEnv(&confer.Opts.InfluxDBConf.UDPAddress)
		confer.replaceByEnv(&confer.Opts.InfluxDBConf.Database)
		confer.replaceByEnv(&confer.Opts.InfluxDBConf.Precision)
		if flushSize := confer.Opts.InfluxDBConf.FlushSize; flushSize == 0 {
			confer.Opts.InfluxDBConf.FlushSize = 20
		}
		if flushTime := confer.Opts.InfluxDBConf.FlushTime; flushTime == 0 {
			confer.Opts.InfluxDBConf.FlushTime = 15
		}
		confer.Opts.InfluxDBConf.Precision = strings.ToLower(confer.Opts.InfluxDBConf.Precision)
		switch confer.Opts.InfluxDBConf.Precision {
		case "ns", "us", "ms", "s", "m", "h":

		default:
			confer.Opts.InfluxDBConf.Precision = "ms"
		}
	}
	// end influxdb

	// healthcheck
	confer.Opts.Heartbeat.API = "http://" + confer.Opts.TrafficInflow.TargetAddress + confer.Opts.Heartbeat.API
	// end healthcheck

	// msp_event
	// ....
	// end msp_event

	// ca start
	confer.replaceByEnv(&confer.Opts.CaConfig.Address)
	confer.replaceByEnv(&confer.Opts.CaConfig.AddressOCSP)
	if confer.Opts.CaConfig.SecurityMode < 1 || confer.Opts.CaConfig.SecurityMode > 2 {
		confer.Opts.CaConfig.SecurityMode = 1
	}
	confer.replaceByEnv(&confer.Opts.CaConfig.AuthKey)
	confer.Opts.MiscroServiceInfo.MetaData["cert_sn"] = ""
	// end ca

	_confer = confer
	return _confer, nil
}

func (*Confer) replaceByEnv(conf *string) {
	if s := os.Getenv(*conf); len(s) > 0 {
		*conf = s
	}
}

func Global() *Confer {
	if _confer == nil {
		return &Confer{}
	}
	return _confer
}

func (confer *Confer) handleSidecarEndpoint() error {
	scheme := HTTP
	if utils.InArray(confer.Opts.TrafficInflow.BindProtocolType, []string{HTTPS, MTLS}) {
		if !confer.Opts.CaConfig.Enable {
			return errors.New("the ca configuration must be enabled in tls mode！")
		}
		scheme = HTTPS
	}
	_, port, err := net.SplitHostPort(confer.Opts.TrafficInflow.BindAddress)
	if err != nil {
		return fmt.Errorf("Traffic Inflow bind address error:%v", err)
	}
	confer.Opts.MiscroServiceInfo.MetaData["mode"] = confer.Opts.TrafficInflow.BindProtocolType
	confer.Opts.MiscroServiceInfo.EndPointAddress = fmt.Sprintf("%s://%s:%s", scheme, confer.Opts.MiscroServiceInfo.PodIP, port)
	return nil
}
