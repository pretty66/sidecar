package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/openmsp/sidecar/pkg/autoredis"
	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/pkg/errno"
	"github.com/openmsp/sidecar/pkg/out"
	"github.com/openmsp/sidecar/sc"
	"github.com/openmsp/sidecar/utils"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	"github.com/openmsp/cilog"
	"github.com/openmsp/kit/ratelimiter"
)

const (
	MspServiceRuleRatelimterRedisFieldKey = "ratelimit"
	LimitServerRedisKey                   = "ratelimit:%s"
	LimitServerAPIRedisKey                = "ratelimit:%s-%s"
)

type RateLimiter struct {
	UniqueID    string
	ServiceName string
	Hostname    string
	rc          *redisCluster
	Timestamp   int64
	confer      confer.RateLimiterS
	limiters    map[string]*ratelimiter.Limiter
	ruleDB      autoredis.AutoClient
	sidecar     *sc.SC
	configChan  <-chan []byte
	lock        sync.RWMutex
}

type rlc struct {
	Max      int  `json:"max"`
	Duration uint `json:"duration"`
}

type rateLimiterConfig struct {
	ServerLimiterIsOpen uint8           `json:"server_limiter_is_open"`
	APILimiterIsOpen    uint8           `json:"api_limiter_is_open"`
	Timestamp           int64           `json:"timestamp"`
	ServerConfig        *rlc            `json:"server_config"`
	APIConfig           map[string]*rlc `json:"api_config"`
}

type redisCluster struct {
	*redis.ClusterClient
}

func (c *redisCluster) RateDel(ctx context.Context, key string) error {
	return c.Del(ctx, key).Err()
}

func (c *redisCluster) RateEvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) (interface{}, error) {
	return c.EvalSha(ctx, sha1, keys, args...).Result()
}

func (c *redisCluster) RateScriptLoad(ctx context.Context, script string) (string, error) {
	var sha1 string
	res, err := c.ScriptLoad(ctx, script).Result()
	if err == nil {
		sha1 = res
	}
	return sha1, err
}

func NewRateLimiter(uniqueID, serviceName, hostname string, rlc confer.RateLimiterS, ruleRedisCluster autoredis.AutoClient, sidecar *sc.SC) *RateLimiter {
	l := &RateLimiter{
		UniqueID:    uniqueID,
		ServiceName: serviceName,
		Hostname:    hostname,
		limiters:    make(map[string]*ratelimiter.Limiter),
		confer:      rlc,
		ruleDB:      ruleRedisCluster,
		sidecar:     sidecar,
	}
	l.rc = &redisCluster{
		redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:              strings.Split(rlc.RedisClusterNodes, ","),
			MaxConnAge:         time.Duration(rlc.MaxConnAge) * time.Second,
			IdleCheckFrequency: time.Duration(rlc.IdleCheckFrequency) * time.Second,
		}),
	}

	l.configChan = l.sidecar.RegisterConfigWatcher(MspServiceRuleRatelimterRedisFieldKey, func(key, value []byte) bool {
		return string(key) == MspServiceRuleRatelimterRedisFieldKey
	}, 1)

	l.listenForConfigurationChanges()
	return l
}

func (l *RateLimiter) HTTPRateLimiter(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := l.Limiter(context.TODO(), c.Request().URL.Path)
		if err != nil {
			return out.HTTPJsonRateLimitError(c.Response(), http.StatusTooManyRequests, err)
		}
		return next(c)
	}
}

func (l *RateLimiter) Limiter(ctx context.Context, api string) *errno.SCError {
	l.lock.RLock()

	if len(l.limiters) == 0 {
		l.lock.RUnlock()
		return nil
	}
	limit, ok := l.limiters[api]
	l.lock.RUnlock()
	if ok {
		res, err := limit.Get(ctx, api)
		if err != nil {
			return errno.ErrLimiter.Add(err.Error())
		}
		if res.Remaining < 0 {
			return errno.ErrLimiter
		}
	}

	l.lock.RLock()
	limit, ok = l.limiters[l.UniqueID]
	l.lock.RUnlock()

	if ok {
		res, err := limit.Get(ctx, l.UniqueID)
		if err != nil {
			return errno.ErrLimiter.Add(err.Error())
		}
		if res.Remaining < 0 {
			return errno.ErrLimiter
		}
	}
	return nil
}

func (l *RateLimiter) updateLimiter(rlc *rateLimiterConfig) error {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.Timestamp = rlc.Timestamp

	var err error
	l.check(rlc)
	if rlc.ServerLimiterIsOpen == 1 {
		if rlc.ServerConfig.Max < 0 {
			delete(l.limiters, l.UniqueID)
		} else {
			l.limiters[l.UniqueID], err = ratelimiter.New(context.TODO(), ratelimiter.Options{
				Max:      rlc.ServerConfig.Max,
				Duration: time.Second * time.Duration(rlc.ServerConfig.Duration),
				Prefix:   fmt.Sprintf(LimitServerRedisKey, l.UniqueID),
				Client:   l.rc,
			})
			if err != nil {
				return err
			}
		}
	} else {
		delete(l.limiters, l.UniqueID)
	}
	if rlc.APILimiterIsOpen == 1 {
		for api, conf := range rlc.APIConfig {
			if conf.Max < 0 {
				delete(l.limiters, api)
			} else {
				l.limiters[api], err = ratelimiter.New(context.TODO(), ratelimiter.Options{
					Max:      conf.Max,
					Duration: time.Second * time.Duration(conf.Duration),
					Prefix:   fmt.Sprintf(LimitServerAPIRedisKey, l.UniqueID, api),
					Client:   l.rc,
				})
				if err != nil {
					return err
				}
			}
		}
	} else {
		for k := range l.limiters {
			if k == l.UniqueID {
				continue
			}
			delete(l.limiters, k)
		}
	}
	return nil
}

func (l *RateLimiter) closeRateLimiter() {
	l.lock.Lock()
	defer l.lock.Unlock()
	if len(l.limiters) > 0 {
		l.limiters = map[string]*ratelimiter.Limiter{}
		cilog.LogInfow(cilog.LogNameSidecar, "disable distributed traffic limiting！")
	}
}

func (l *RateLimiter) check(rlConfig *rateLimiterConfig) {
	if rlConfig.ServerLimiterIsOpen > 0 {
		if rlConfig.ServerConfig.Max == 0 {
			rlConfig.ServerConfig.Max = l.confer.Max
		}
		if rlConfig.ServerConfig.Duration == 0 {
			rlConfig.ServerConfig.Duration = l.confer.Duration
		}
	}
	if rlConfig.APILimiterIsOpen > 0 {
		for key, val := range rlConfig.APIConfig {
			if val.Max == 0 {
				rlConfig.APIConfig[key].Max = l.confer.APIMax
			}
			if val.Duration == 0 {
				rlConfig.APIConfig[key].Duration = l.confer.APIDuration
			}
		}
	}
}

func (l *RateLimiter) listenForConfigurationChanges() {
	utils.GoWithRecover(func() {
		for data := range l.configChan {
			if len(data) == 0 {
				l.closeRateLimiter()
				continue
			}
			l.lock.RLock()
			lastUpdate := l.Timestamp
			l.lock.RUnlock()

			rlConfig := new(rateLimiterConfig)
			err := json.Unmarshal(data, rlConfig)
			if err != nil {
				cilog.LogErrorw(cilog.LogNameSidecar, "ratelimiter getConfigByRedis ", err)
				continue
			}

			if rlConfig.Timestamp <= lastUpdate {
				continue
			}
			err = l.updateLimiter(rlConfig)
			if err != nil {
				cilog.LogErrorw(cilog.LogNameSidecar, "Failed to update the traffic limiting rule. Procedure ", err)
			} else {
				b, _ := json.Marshal(rlConfig)
				cilog.LogInfof(cilog.LogNameSidecar, "The configuration of the induction current limiting rule changes:%s", b)
			}
		}
	}, func(_ interface{}) {
		time.Sleep(time.Second)
		l.listenForConfigurationChanges()
	})
}
