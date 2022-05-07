package remoteconfer

import (
	"context"
	"errors"
	"fmt"
	"github.com/openmsp/sidecar/pkg/autoredis"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/openmsp/cilog"
	"github.com/openmsp/kit/rediscluster"
)

const (
	MspServiceRuleRedisKey  = "msp:service_rule:%s"
	MspConfigLastUpdateTime = "timestamp"
	defaultCycle            = 21

	MspServiceRuleRedisProxyKey = "redis_proxy"
	MspServiceRuleFuseKey       = "fuse"
	MspServiceRuleACL           = "TrafficTarget"
)

type ConfigAcceptor func(key, value []byte) bool

type configWatcher struct {
	pred ConfigAcceptor
	ch   chan<- []byte
}

type FetcherConfig struct {
	IsWatchEnabled bool
	HostName       string
	StreamKey      string
}

type RemoteConfigFetcher struct {
	lock           sync.Mutex
	redisCluster   autoredis.AutoClient
	ctx            context.Context
	uniqueID       string
	configWatchers map[string]*configWatcher
	cycleInSeconds int
	lastUpdateTime *int64
	conf           FetcherConfig
	watchChan      chan bool
	closed         bool
	notice         bool
}

func NewConfigFetcher(ctx context.Context, redisCluster autoredis.AutoClient, uniqueID string, cycle int, config FetcherConfig) *RemoteConfigFetcher {
	if cycle == 0 {
		cycle = defaultCycle
	}
	if config.IsWatchEnabled && cycle < defaultCycle {
		cycle = defaultCycle
		cilog.LogInfof(cilog.LogNameSidecar, "Enable configuration change listening cycle is smaller than the default cycle，change the cycle to %d", cycle)
	}
	r := &RemoteConfigFetcher{
		uniqueID:       uniqueID,
		cycleInSeconds: cycle,
		ctx:            ctx,
		redisCluster:   redisCluster,
		configWatchers: make(map[string]*configWatcher),
		conf:           config,
		lastUpdateTime: new(int64),
		watchChan:      make(chan bool, 1),
	}
	go func() {
		if ctx != nil {
			if <-ctx.Done(); true {
				r.Close()
			}
		}
	}()
	return r
}

func (r *RemoteConfigFetcher) AddWatcher(key string, c ConfigAcceptor, buffer int) <-chan []byte {
	r.lock.Lock()
	if r.closed {
		r.lock.Unlock()
		ch := make(chan []byte)
		close(ch)
		return ch
	}
	if buffer <= 0 {
		buffer = 1
	}
	configChan := make(chan []byte, buffer)
	ch := &configWatcher{
		pred: c,
		ch:   configChan,
	}
	r.configWatchers[key] = ch
	r.lock.Unlock()
	return configChan
}

func (r *RemoteConfigFetcher) Start() {
	r.getConfigByRedis(r.uniqueID)

	if r.conf.IsWatchEnabled {
		r.startWithWatch()
	} else {
		r.start(1)
	}
}

func (r *RemoteConfigFetcher) start(interval int) {
	d := time.Duration(interval) * time.Second
	timer := time.NewTicker(d)
	defer timer.Stop()
	for {
		r.lock.Lock()
		if r.closed {
			r.lock.Unlock()
			return
		}
		r.lock.Unlock()
		select {
		case <-timer.C:
			r.getConfigByRedis(r.uniqueID)
		case <-r.ctx.Done():
			return
		}
	}
}

func (r *RemoteConfigFetcher) startWithWatch() {
	go r.start(r.cycleInSeconds)
	for {
		r.lock.Lock()
		if r.closed {
			r.lock.Unlock()
			return
		}
		r.lock.Unlock()
		needFetch := r.watchConfig(r.uniqueID)
		if needFetch {
			r.getConfigByRedis(r.uniqueID)
		}
	}
}

func sleep() {
	// time.Sleep(3*time.Second + time.Duration(rand.Int()%1000)*time.Millisecond)
	time.Sleep(10 * time.Second)
}

func (r *RemoteConfigFetcher) watchConfig(uniqueID string) (needFetch bool) {
	updateAt := "$"
	if r.loadLastUpdateTime() > 0 {
		updateAt = strconv.FormatInt(r.loadLastUpdateTime(), 10)
	}
	redisKey := fmt.Sprintf(MspServiceRuleRedisKey, uniqueID)
	// k
	res, err := r.redisCluster.WConfig(context.Background(), &redis.WConfigArgs{
		Configs:    []string{redisKey, updateAt},
		StreamKey:  r.conf.StreamKey,
		ConsumerID: r.conf.HostName,
		Block:      time.Duration(r.cycleInSeconds) * time.Second,
	})
	if err != nil {
		if errors.Is(err, rediscluster.ErrNil) || errors.Is(err, context.Canceled) {
			return false
		}
		sleep()
		if errors.Is(err, redis.ErrClosed) {
			return
		}
		cilog.LogWarnf(cilog.LogNameSidecar, "Failed to listen service configuration from Redis Proxy err: %s", err.Error())
		return false
	}
	if len(res.Values) == 0 {
		return true
	}
	keyList := r.getConfigWatchersKey()

	for key, value := range res.Values {
		keyByt := []byte(key)

		valueByt := []byte(value)
		delete(keyList, key)
		if key == MspConfigLastUpdateTime {
			lastUpdateTime, err := rediscluster.Int64(valueByt, nil)
			if err == nil {
				r.setLastUpdateTime(lastUpdateTime)
			}
			continue
		}
		r.handleConfig(keyByt, valueByt)
	}
	for key := range keyList {
		r.handleConfig([]byte(key), nil)
	}
	cilog.LogInfof(cilog.LogNameSidecar, "The configuration listener detects the unique ID of the rule configuration change：%s set the last update time：%d", uniqueID, r.loadLastUpdateTime())
	return false
}

func (r *RemoteConfigFetcher) loadLastUpdateTime() int64 {
	return atomic.LoadInt64(r.lastUpdateTime)
}

func (r *RemoteConfigFetcher) setLastUpdateTime(val int64) {
	atomic.StoreInt64(r.lastUpdateTime, val)
}

func (r *RemoteConfigFetcher) getConfigByRedis(uniqueID string) {
	redisKey := fmt.Sprintf(MspServiceRuleRedisKey, uniqueID)
	lastUpdateTime, err := rediscluster.Int64(r.redisCluster.Do("HGET", redisKey, MspConfigLastUpdateTime))
	if err != nil {
		if err == rediscluster.ErrNil {
			if r.notice {
				keyList := r.getConfigWatchersKey()
				for key := range keyList {
					r.handleConfig([]byte(key), nil)
				}
				r.notice = false
			}
			return
		}
		cilog.LogErrorw(cilog.LogNameSidecar, "The last update time of the pull configuration is incorrect", err)
		return
	}
	r.notice = true
	if r.loadLastUpdateTime() >= lastUpdateTime {
		return
	}
	r.setLastUpdateTime(lastUpdateTime)

	data, err := rediscluster.Values(r.redisCluster.Do("HSCAN", redisKey, 0, "COUNT", 50))
	if err != nil && !errors.Is(err, rediscluster.ErrNil) && !errors.Is(err, redis.ErrClosed) {
		cilog.LogWarnf(cilog.LogNameSidecar, "failed to get configuration from redis cluster err: %s", err.Error())
		return
	}
	if len(data) != 2 { // this should not happen
		cilog.LogWarnw(cilog.LogNameSidecar, "Get configuration from Redis Cluster and get unexpected return")
		return
	}
	if cursor, err := rediscluster.Int(data[0], nil); err != nil || cursor != 0 {
		cilog.LogWarnw(cilog.LogNameSidecar, "There are too many configuration item types. Check whether there are any problems")
		return
	}
	valueArrayI, ok := data[1].([]interface{})
	if !ok {
		return
	}
	keyList := r.getConfigWatchersKey()
	for i := 0; i < len(valueArrayI)/2; i++ {
		keyByt, err := rediscluster.Bytes(valueArrayI[2*i], nil)
		if err != nil {
			continue
		}
		valueByt, err := rediscluster.Bytes(valueArrayI[2*i+1], nil)
		if err != nil {
			continue
		}
		delete(keyList, string(keyByt))
		r.handleConfig(keyByt, valueByt)
	}
	for key := range keyList {
		r.handleConfig([]byte(key), nil)
	}
}

func (r *RemoteConfigFetcher) handleConfig(key, value []byte) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.closed {
		return
	}
	watcher, ok := r.configWatchers[string(key)]
	if ok && watcher.pred(key, value) {
		select {
		case watcher.ch <- value:
		default:
			if !r.closed {
				cilog.LogWarnf(cilog.LogNameSidecar, "The distribution configuration failed, and the receiver cannot process the distribution：%s", string(key))
			}
		}
	}
}

func (r *RemoteConfigFetcher) Close() {
	r.lock.Lock()
	if r.closed {
		r.lock.Unlock()
		return
	}
	r.closed = true
	for _, ch := range r.configWatchers {
		close(ch.ch)
	}
	r.configWatchers = nil
	r.lock.Unlock()
}

func (r *RemoteConfigFetcher) getConfigWatchersKey() map[string]struct{} {
	r.lock.Lock()
	defer r.lock.Unlock()
	m := make(map[string]struct{}, len(r.configWatchers))
	for key := range r.configWatchers {
		if key == MspConfigLastUpdateTime {
			continue
		}
		m[key] = struct{}{}
	}
	return m
}
