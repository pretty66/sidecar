package autoredis

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/utils"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/ztalab/ZACA/pkg/caclient"

	"github.com/openmsp/cilog"

	"github.com/go-redis/redis/v8"
	"github.com/openmsp/kit/rediscluster"
	"github.com/openmsp/sesdk/discovery"
)

const (
	TypeDirectNode    = "node"
	TypeDirectCluster = "cluster"
	TypeProxy         = "proxy"
)

var ErrNotImpl = errors.New("not impl")

type AutoClient interface {
	Do(cmd string, args ...interface{}) (interface{}, error)
	WConfig(ctx context.Context, a *redis.WConfigArgs) (redis.WConfig, error)
	IsTypeProxy() bool
	IsNode() bool
	GetClient() interface{}
	Close()
}

type Client struct {
	Type          string
	clock         sync.RWMutex
	nodeClient    *redis.Client
	clusterClient *rediscluster.Cluster
	ctx           context.Context
	cancel        context.CancelFunc
	conf          confer.RedisClusterConf
	discovery     *discovery.Discovery
	e             *caclient.Exchanger
}

func NewAutoRedisClient(ctx context.Context, discovery *discovery.Discovery, conf confer.RedisClusterConf, e *caclient.Exchanger) (c *Client, err error) {
	c = &Client{conf: conf, discovery: discovery, e: e}
	switch {
	case c.conf.Type == TypeProxy || c.conf.UseProxy:
		c.Type = TypeProxy
		err = c.ConnectProxy()
		if err != nil {
			return nil, err
		}
	case c.conf.Type == TypeDirectNode:
		c.Type = TypeDirectNode
		c.nodeClient = redis.NewClient(&redis.Options{
			Addr:         c.conf.StartNodes,
			MinIdleConns: 2,
			DialTimeout:  time.Duration(c.conf.ConnTimeOut) * time.Second,
			ReadTimeout:  time.Duration(c.conf.ConnReadTimeOut) * time.Second,
			WriteTimeout: time.Duration(c.conf.ConnWriteTimeOut) * time.Second,
			PoolSize:     c.conf.ConnPoolSize,
			MaxConnAge:   time.Duration(c.conf.ConnAliveTimeOut) * time.Second,
		})
		err = c.nodeClient.Ping(context.Background()).Err()
		if err != nil {
			return nil, err
		}
	default:
		c.Type = TypeDirectCluster
		c.clusterClient, err = rediscluster.NewCluster(
			&rediscluster.Options{
				StartNodes:             strings.Split(conf.StartNodes, ","),
				ConnTimeout:            time.Duration(conf.ConnTimeOut) * time.Second,
				ReadTimeout:            time.Duration(conf.ConnReadTimeOut) * time.Second,
				WriteTimeout:           time.Duration(conf.ConnWriteTimeOut) * time.Second,
				KeepAlive:              conf.ConnPoolSize,
				AliveTime:              time.Duration(conf.ConnAliveTimeOut) * time.Second,
				SlaveOperateRate:       conf.SlaveOperateRate,
				ClusterUpdateHeartbeat: conf.ClusterUpdateHeartbeat,
			})
		if err != nil {
			return nil, err
		}
	}
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.switchConnect()
	return c, nil
}

func (c *Client) Ping() error {
	if c.IsNode() {
		_, err := c.nodeClient.Ping(c.ctx).Result()
		return err
	}
	_, err := c.clusterClient.Do("PING")
	return err
}

func (c *Client) Do(cmd string, args ...interface{}) (data interface{}, err error) {
	if c.IsNode() {
		doargs := make([]interface{}, 1, len(args)+1)
		doargs[0] = cmd
		doargs = append(doargs, args...)
		c.clock.RLock()
		client := c.nodeClient
		c.clock.RUnlock()
		data, err = client.Do(c.ctx, doargs...).Result()
		if err == redis.Nil {
			return nil, rediscluster.ErrNil
		}
	} else {
		c.clock.RLock()
		client := c.clusterClient
		c.clock.RUnlock()
		data, err = client.Do(cmd, args...)
		if err == rediscluster.ErrNil {
			return data, err
		}
	}
	if err != nil && c.IsTypeProxy() {
		cilog.LogInfof(cilog.LogNameSidecar, "do cmd %s err %s", cmd, err.Error())
		if err == redis.ErrClosed || err == context.Canceled {
			return
		}
		e := c.ConnectProxy()
		if e != nil {
			cilog.LogErrorw(cilog.LogNameSidecar, "Reconnect proxy error：", e)
		}
	}
	return
}

func (c *Client) Close() {
	c.cancel()
}

func (c *Client) IsNode() bool {
	return c.Type == TypeDirectNode || c.Type == TypeProxy
}

func (c *Client) IsTypeProxy() bool {
	return c.Type == TypeProxy
}

func (c *Client) ConnectProxy() error {
	if !c.IsTypeProxy() {
		return errors.New("non proxy mode access！")
	}
	ep, err := c.discovery.GetEndpoint(c.conf.ProxyUniqueID)
	if err != nil {
		if errors.Is(discovery.ErrInstanceNotFound, err) {
			return fmt.Errorf("getting redis cluster proxy error！")
		}
		return err
	}

	var tlsc *tls.Config
	if ep.Mode == confer.MTLS {
		if c.e == nil {
			return errors.New("need mtls to redis but ca not enabled")
		}
		generator, e := c.e.ClientTLSConfig("")
		if e != nil {
			return err
		}
		tlsc = generator.TLSConfig()
	}
	cilog.LogInfof(cilog.LogNameSidecar, "Auto Redis reconnects the proxy connection to close the old client and rebuild the new one. New node address：%s", ep.Host)
	nodeClient := redis.NewClient(&redis.Options{
		Addr:         ep.Host,
		TLSConfig:    tlsc,
		MinIdleConns: 2,
		DialTimeout:  time.Duration(c.conf.ConnTimeOut) * time.Second,
		ReadTimeout:  time.Duration(c.conf.ConnReadTimeOut) * time.Second,
		WriteTimeout: time.Duration(c.conf.ConnWriteTimeOut) * time.Second,
		PoolSize:     c.conf.ConnPoolSize,
		MaxConnAge:   time.Duration(c.conf.ConnAliveTimeOut) * time.Second,
	})
	err = nodeClient.Ping(context.Background()).Err()
	if err != nil {
		return err
	}

	c.clock.Lock()
	c.closeConnect(c.nodeClient, c.clusterClient, false)
	c.nodeClient = nodeClient
	c.clock.Unlock()
	return nil
}

func (c *Client) WConfig(ctx context.Context, a *redis.WConfigArgs) (data redis.WConfig, err error) {
	if !c.IsTypeProxy() {
		return redis.WConfig{}, ErrNotImpl
	}
	c.clock.RLock()
	client := c.nodeClient
	c.clock.RUnlock()
	data, err = client.WConfig(ctx, a).Result()
	if err == redis.Nil {
		return redis.WConfig{}, rediscluster.ErrNil
	}
	if err != nil && c.IsTypeProxy() && err != context.Canceled {
		if err == redis.ErrClosed {
			return
		}
		cilog.LogInfof(cilog.LogNameSidecar, "wconfig err %s", err.Error())
		e := c.ConnectProxy()
		if e != nil {
			cilog.LogErrorw(cilog.LogNameSidecar, "Reconnect proxy error：", e)
		}
	}
	return
}

func (c *Client) GetClient() interface{} {
	if c.IsNode() {
		return c.nodeClient
	}
	return c.clusterClient
}

func (c *Client) closeConnect(nodeCli *redis.Client, clusterCli *rediscluster.Cluster, isNow bool) {
	if isNow {
		if clusterCli != nil {
			clusterCli.Close()
		}
		if nodeCli != nil {
			_ = nodeCli.Close()
		}
		return
	}
	utils.GoWithRecover(func() {
		time.Sleep(time.Second * 30)
		if clusterCli != nil {
			clusterCli.Close()
		}
		if nodeCli != nil {
			_ = nodeCli.Close()
		}
	}, nil)
}

func (c *Client) switchConnect() {
	utils.GoWithRecover(func() {
		if c.IsTypeProxy() && c.conf.TimingSwitchProxy {
			log.Printf("Enable proxy timing switchover, unit：%dseconds", c.conf.SwitchProxyCycle)
		}
		for {
			if c.IsTypeProxy() && c.conf.TimingSwitchProxy {
				select {
				case <-time.After(time.Duration(c.conf.SwitchProxyCycle) * time.Second):
					_ = c.ConnectProxy()
				case <-c.ctx.Done():
					c.clock.Lock()
					c.closeConnect(c.nodeClient, c.clusterClient, true)
					c.clock.Unlock()
					return
				}
			} else {
				select {
				case <-c.ctx.Done():
					c.clock.Lock()
					c.closeConnect(c.nodeClient, c.clusterClient, true)
					c.clock.Unlock()
					return
				}
			}
		}
	}, func(r interface{}) {
		time.Sleep(time.Second)
		cilog.LogWarnw(cilog.LogNameSidecar, "The scheduled reconnection redis agent is abnormal. Procedure，restart...")
		c.switchConnect()
	})
}
