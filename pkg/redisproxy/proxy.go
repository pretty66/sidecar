package redisproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/openmsp/sidecar/pkg/autoredis"
	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/pkg/remoteconfer"
	"github.com/openmsp/sidecar/sc"
	"github.com/openmsp/sidecar/utils"
	"github.com/openmsp/sidecar/utils/bareneter"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ztalab/ZACA/pkg/caclient"

	"github.com/IceFireDB/IceFireDB-Proxy/pkg/router/redisCluster"
	"github.com/openmsp/kit/rediscluster"

	"github.com/IceFireDB/IceFireDB-Proxy/pkg/router/redisNode"
	"github.com/go-redis/redis/v8"

	"github.com/IceFireDB/IceFireDB-Proxy/pkg/router"
	"github.com/openmsp/cilog"
)

// Control plane delivery configuration format
type ProxyConfig struct {
	confer.RedisClusterConf
	Namespace string `json:"namespace"`
	Timestamp int64  `json:"timestamp"`
	LocalPort int    `json:"local_port"`
}

type Proxy struct {
	ctx        context.Context
	clock      sync.Mutex
	conf       *ProxyConfig
	server     *bareneter.Server
	rlock      sync.RWMutex
	router     router.IRoutes
	close      atomic.Value
	sidecar    *sc.SC
	configChan <-chan []byte
	stopProxy  chan struct{}
	reconn     chan struct{}
	client     *autoredis.Client
	e          *caclient.Exchanger
}

func Start(ctx context.Context, e *caclient.Exchanger) {
	p := &Proxy{ctx: ctx, sidecar: sc.Sc(), e: e, stopProxy: make(chan struct{}), reconn: make(chan struct{})}
	p.close.Store(true)
	p.configChan = p.sidecar.RegisterConfigWatcher(remoteconfer.MspServiceRuleRedisProxyKey, func(key, value []byte) bool {
		return string(key) == remoteconfer.MspServiceRuleRedisProxyKey
	}, 1)
	go func() {
		select {
		case <-ctx.Done():
			if !p.closed() {
				p.closeProxy()
				cilog.LogWarnw(cilog.LogNameSidecar, "the sidecar cache proxy function is disabled")
			}
		}
	}()
	p.recoverListenConfig()
	p.reConnectRemote()
}

func (p *Proxy) recoverListenConfig() {
	utils.GoWithRecover(func() {
		p.listenConfig()
	}, func(r interface{}) {
		p.recoverListenConfig()
	})
}

func (p *Proxy) listenConfig() {
	for conf := range p.configChan {
		if len(conf) == 0 {
			if !p.closed() {
				p.closeProxy()
				cilog.LogWarnw(cilog.LogNameSidecar, "the sidecar cache proxy function is disabled")
			}
			continue
		}
		var config *ProxyConfig
		err := json.Unmarshal(conf, &config)
		if err != nil {
			cilog.LogErrorw(cilog.LogNameSidecar, "redis proxy configuration resolution error："+string(conf), err)
			continue
		}
		if config == nil {
			cilog.LogError(cilog.LogNameSidecar, "redis proxy configuration resolution error："+string(conf))
			continue
		}
		time.Sleep(time.Second)
		if !p.closed() {
			p.clock.Lock()
			timestamp := p.conf.Timestamp
			p.clock.Unlock()
			if config.Timestamp <= timestamp {
				continue
			}
		} else if !config.Enable {
			continue
		}
		if err = config.validate(); err != nil {
			cilog.LogErrorw(cilog.LogNameSidecar, "configuration verification failed：", err)
			continue
		}
		p.clock.Lock()
		p.conf = config
		p.clock.Unlock()

		cilog.LogInfow(cilog.LogNameSidecar, "redis proxy configuration changed："+string(conf))
		utils.GoWithRecover(func() {
			p.reloadRedisProxy()
		}, nil)
	}
}

func (pc *ProxyConfig) validate() error {
	if pc.UseProxy {
		if len(pc.ProxyUniqueID) == 0 {
			return errors.New("proxy middleware unique id error")
		}
	} else {
		if len(pc.StartNodes) == 0 {
			return errors.New("cluster node error")
		}
	}
	if len(pc.Namespace) == 0 {
		return errors.New("the key prefix must be cached")
	}
	if pc.ConnTimeOut <= 0 {
		pc.ConnTimeOut = confer.Global().Opts.RedisClusterConf.ConnTimeOut
	}
	if pc.ConnAliveTimeOut <= 0 {
		pc.ConnAliveTimeOut = confer.Global().Opts.RedisClusterConf.ConnAliveTimeOut
	}
	if pc.ConnReadTimeOut <= 0 {
		pc.ConnReadTimeOut = confer.Global().Opts.RedisClusterConf.ConnReadTimeOut
	}
	if pc.ConnWriteTimeOut <= 0 {
		pc.ConnWriteTimeOut = confer.Global().Opts.RedisClusterConf.ConnWriteTimeOut
	}
	if pc.LocalPort <= 0 {
		return errors.New("local listening address error")
	}
	return nil
}

func (p *Proxy) closed() bool {
	return p.close.Load().(bool)
}

func (p *Proxy) closeProxy() {
	p.rlock.Lock()
	_ = p.router.Close()
	p.rlock.Unlock()
	_ = p.server.Close()
}

func (p *Proxy) reloadRedisProxy() {
	if !p.closed() {
		p.closeProxy()
	}
	p.clock.Lock()
	if !p.conf.Enable {
		p.clock.Unlock()
		return
	}
	p.clock.Unlock()
	route := p.newConnectRemote(p.conf.RedisClusterConf)
	if route == nil {
		return
	}
	route.Use(router.Namespace([]byte(p.conf.Namespace)))
	route.InitCMD()
	p.rlock.Lock()
	p.router = route
	p.rlock.Unlock()
	p.listenLocalPort(p.conf.LocalPort)
}

func (p *Proxy) newConnectRemote(redisConf confer.RedisClusterConf) router.IRoutes {
	var client *autoredis.Client
	var err error
	for i := 0; i < 3; i++ {
		client, err = autoredis.NewAutoRedisClient(p.ctx, p.sidecar.Discovery, redisConf, p.e)
		if err != nil {
			cilog.LogInfof(cilog.LogNameSidecar, "sidecar The Redis proxy client failed to establish a remote connection. Procedure error：%s", err.Error())
			time.Sleep(time.Millisecond * 300)
			continue
		}
		break
	}
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "redis proxy proxy target connection error", err)
		return nil
	}

	p.client = client
	if p.client.IsNode() {
		cli, ok := p.client.GetClient().(*redis.Client)
		if !ok {
			cilog.LogError(cilog.LogNameSidecar, "error obtaining proxy client")
			return nil
		}
		return redisNode.NewRouter(cli)
	}
	cli, ok := p.client.GetClient().(*rediscluster.Cluster)
	if !ok {
		cilog.LogError(cilog.LogNameSidecar, "error obtaining proxy client")
		return nil
	}
	return redisCluster.NewRouter(cli)
}

func (p *Proxy) reConnectRemote() {
	utils.GoWithRecover(func() {
		for {
			time.Sleep(3 * time.Second)
			select {
			case <-p.reconn:
				if p.client == nil || !p.client.IsTypeProxy() {
					continue
				}
				route := p.newConnectRemote(p.conf.RedisClusterConf)
				if route == nil {
					continue
				}

				route.InitCMD()
				p.rlock.Lock()
				_ = p.router.Close()
				p.router = route
				p.rlock.Unlock()
				cilog.LogWarnw(cilog.LogNameSidecar, "redis proxy reconnect to the target redis")
			}
		}
	}, func(r interface{}) {
		p.reConnectRemote()
	})
}

func (p *Proxy) listenLocalPort(port int) {
	if !p.closed() {
		panic("BUG: we have to shut down the original listening！")
	}
	p.server = bareneter.NewServerNetwork("tcp", fmt.Sprintf(":%d", port),
		p.handle,
		p.accept,
		p.connClosed)
	p.close.Store(false)
	defer p.close.Store(true)
	e := p.server.ListenAndServe()
	if e != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "redis proxy listening port error", e)
	}
}
