package trace

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/openmsp/sidecar/utils"
	"github.com/openmsp/sidecar/utils/skyapmagent/service"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SkyAPM/go2sky"
	"github.com/SkyAPM/go2sky/reporter"
	"github.com/openmsp/cilog"
)

const (
	typeApm   = "apm"
	typeTrace = "trace"
	typeAuto  = "auto"
)

type Trace struct {
	config     Config
	skyAddress string
	grpcReport go2sky.Reporter
	apm        *service.Agent
	sky        *SkyTrace
	configChan <-chan []byte
	close      atomic.Value
	cancel     context.CancelFunc
	lock       sync.Mutex
}

type Config struct {
	Enable       bool   `json:"enable"`
	Type         string `json:"type"`
	ConfigOfType string `json:"config_of_type"`
	Timestamp    int64  `json:"timestamp"`
}

type apmConfig struct {
	// local agent listen net type
	AgentListenNetType string `yaml:"agent_listen_net_type" json:"agent_listen_net_type"`
	// local agent listen net address
	AgentListenNetURI string `yaml:"agent_listen_net_uri" json:"agent_listen_net_uri"`
	// apm agent data forwarding rate：Send trace 1 second by default
	AgentSendRate int `yaml:"agent_send_rate" json:"agent_send_rate"`
	// The maximum KB size for trace parsing. 0 or -1 disables large trace parsing
	AgentTrimMaxSize int `json:"agent_trim_max_size"`
}

var defaultApmConfig = apmConfig{
	AgentListenNetType: "unix",
	AgentListenNetURI:  "/sidecar/sky-agent.sock",
	AgentSendRate:      100,
	AgentTrimMaxSize:   10 * 1024, // 10 M
}

func NewTrace(SkyCollectorGrpcAddress string, c Config, configChan <-chan []byte) (t *Trace, err error) {
	if c.Enable && SkyCollectorGrpcAddress == "" {
		return nil, errors.New("SkyCollectorGrpcAddress is empty")
	}
	t = &Trace{
		config:     c,
		skyAddress: SkyCollectorGrpcAddress,
	}
	t.configChan = configChan
	if !c.Enable {
		t.sky = NewSkyTrace(false, nil)
		t.close.Store(true)
		return t, nil
	}
	t.close.Store(false)
	r, err := reporter.NewGRPCReporter(SkyCollectorGrpcAddress,
		reporter.WithCheckInterval(time.Second*40),
		reporter.WithMaxSendQueueSize(65535*2),
	)
	if err != nil {
		return nil, err
	}
	t.grpcReport = r
	t.sky = NewSkyTrace(c.Type != typeApm, t.grpcReport)
	err = t.sky.Init()
	if err != nil {
		return nil, err
	}
	var closeTraceFunc func() = nil
	switch c.Type {
	case typeAuto:
		closeTraceFunc = t.closeTrace
		fallthrough
	case typeApm:
		if t.apm != nil {
			t.apm.Close()
		}
		apmConf := defaultApmConfig
		if c.ConfigOfType != "" {
			_ = json.Unmarshal([]byte(c.ConfigOfType), &apmConf)
		}
		ctx, cancel := context.WithCancel(context.Background())
		t.cancel = cancel
		t.apm = service.NewAgent(
			ctx,
			8,
			SkyCollectorGrpcAddress,
			apmConf.AgentListenNetType,
			apmConf.AgentListenNetURI,
			apmConf.AgentSendRate,
			apmConf.AgentTrimMaxSize,
			closeTraceFunc,
			t.grpcReport,
		)
		go t.apm.Run()
	case typeTrace:
		fallthrough
	default:
	}

	go t.listenConfig()
	return t, nil
}

func (t *Trace) listenConfig() {
	utils.GoWithRecover(func() {
		t.listenForConfigurationChanges()
	}, func(_ interface{}) {
		time.Sleep(time.Second)
		t.listenConfig()
	})
}

func (t *Trace) listenForConfigurationChanges() {
	for data := range t.configChan {
		if len(data) == 0 {
			continue
		}
		var redisConfig Config

		err := json.Unmarshal(data, &redisConfig)
		if err != nil {
			cilog.LogErrorw(cilog.LogNameSidecar, "trace getConfigByRedis ", err)
			continue
		}
		if redisConfig.Timestamp <= t.config.Timestamp {
			continue
		}
		if redisConfig == t.config {
			continue
		}
		err = t.applyConfig(redisConfig)
		if err != nil {
			cilog.LogErrorw(cilog.LogNameSidecar, "trace apply config err: ", err)
		} else {
			cilog.LogInfof(cilog.LogNameSidecar, "trace apply dynamic config %s", string(data))
		}
	}
}

func (t *Trace) applyConfig(c Config) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	if !c.Enable {
		t.sky.Reload(false, nil)
		t.Close()
		return nil
	}

	r, err := reporter.NewGRPCReporter(t.skyAddress,
		reporter.WithCheckInterval(time.Second*40),
		reporter.WithMaxSendQueueSize(65535*2),
	)
	if err != nil {
		return err
	}
	defer func() {
		if t.grpcReport != nil {
			t.grpcReport.Close()
		}
		t.grpcReport = r
		t.config = c
		t.close.Store(false)
	}()
	t.sky.Reload(c.Type != typeApm, r)
	var closeTraceFunc func() = nil
	switch c.Type {
	case typeAuto:
		closeTraceFunc = t.closeTrace
		fallthrough
	case typeApm:
		if t.apm != nil {
			t.apm.Close()
		}
		apmConf := defaultApmConfig
		if c.ConfigOfType != "" {
			_ = json.Unmarshal([]byte(c.ConfigOfType), &apmConf)
		}
		// ctx, cancel := context.WithCancel(context.Background())
		// t.cancel = cancel
		t.apm = service.NewAgent(
			context.Background(),
			8,
			t.skyAddress,
			apmConf.AgentListenNetType,
			apmConf.AgentListenNetURI,
			apmConf.AgentSendRate,
			apmConf.AgentTrimMaxSize,
			closeTraceFunc,
			r,
		)
		go t.apm.Run()

	case typeTrace:
		t.stopApm()
	default:
		cilog.LogErrorw(cilog.LogNameSidecar, "trace apply config unknown type: ", err)
	}

	return nil
}

func (t *Trace) closeTrace() {
	if t.sky != nil {
		t.sky.Close()
	}
}

func (t *Trace) stopApm() {
	if t.apm != nil {
		t.apm.Stop()
	}
}

func (t *Trace) init() {
}

// never return nil
func (t *Trace) GetSky() *SkyTrace {
	return t.sky
}

func (t *Trace) dynamicConfig() {
}

func (t *Trace) IsClosed() bool {
	return t.close.Load().(bool)
}

func (t *Trace) Close() {
	t.close.Store(true)
	if t.apm != nil {
		t.apm.Close()
	}
	if t.sky != nil && !t.sky.IsClosed() {
		t.sky.Close()
	}
	if t.grpcReport != nil {
		t.grpcReport.Close()
		t.grpcReport = nil
	}
}
