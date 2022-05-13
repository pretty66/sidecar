package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/openmsp/sidecar/sc"
	util "github.com/openmsp/sidecar/utils"

	"github.com/go-redis/redis/v8"
	"github.com/openmsp/cilog"
	"github.com/openmsp/kit/rediscluster"
	"github.com/openmsp/sesdk"
)

const (
	RouterConfigKey         = "msp:service_router:%s"
	RouterConfigKeyPrefix   = "msp:service_router:"
	RouterConfigField       = "config"
	RouterConfigUpdateField = "timestamp"
)

// flow traction topic structure
type Rule struct {
	lock   sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	// uniqueID slb, A target service can be configured with multiple matching rules
	ruleList map[string]UniqueIDRule
	// Initialize traffic mirroring
	onceInitShadow sync.Once
	// mirror traffic queue
	shadowRequestQueue chan *http.Request
	httpClient         *http.Client
	// Incorrect configuration record, if the last update time corresponding
	// to the uniqueID in this list does not change, the configuration will not be pulled
	errorConfigList sync.Map
	// Record last update time unique_id: time
	lastUpdateTimes map[string]int64
	// enable watch
	enableWatch bool
	// Update signal monitoring immediately
	refresh chan struct{}
}

type UniqueIDRule struct {
	lastUpdateTime int64
	entity         []*RuleEntity
	conf           *Config
}

// rules of the entity
type RuleEntity struct {
	Match        Matchable
	Distribution *Distribution
	Mirror       *ShadowRequest
}

func (r *RuleEntity) String() string {
	b, _ := json.Marshal(r)
	return string(b)
}

// creating a route traction
func InitRouterRedirect(ctx context.Context, enableWatch bool) *Rule {
	direct := &Rule{
		ruleList:        map[string]UniqueIDRule{},
		lastUpdateTimes: map[string]int64{},
		enableWatch:     enableWatch,
		refresh:         make(chan struct{}, 1),
	}
	direct.ctx, direct.cancel = context.WithCancel(ctx)
	return direct
}

func (r *Rule) initMirror() {
	r.shadowRequestQueue = make(chan *http.Request, 500)
	r.httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   time.Second,
				KeepAlive: time.Second * 60,
			}).DialContext,
			MaxIdleConns:        500,
			MaxIdleConnsPerHost: 200,
			MaxConnsPerHost:     200,
			IdleConnTimeout:     time.Second * 60,
		},
		Timeout: time.Second * 2,
	}
	go r.sendShadowRequest()
}

func (r *Rule) ListenConfig() {
	util.GoWithRecover(func() {
		for {
			select {
			case <-r.ctx.Done():
				return
			case event := <-sc.Discovery().InstancesDataEvent:
				if !r.Has(event.AppID) {
					r.refresh <- struct{}{}
					continue
				}
				if event.Event == sesdk.UpdateInstance || event.Event == sesdk.SubscribeInstance {
					r.updateEntityRuleByDiscovery(event.AppID, event.Instances)
				} else if event.Event == sesdk.UnsubscribeInstance {
					r.deleteEntity(event.AppID)
				}
			}
		}
	}, nil)

	if r.enableWatch {
		r.listenConfigWithWatch()
	} else {
		r.listenConfig(5)
	}
}

func (r *Rule) listenConfig(count int) {
	tick := time.NewTicker(time.Second * time.Duration(count))
	defer tick.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-tick.C:
			r.BatchGetRouterConfig()
		}
	}
}

func (r *Rule) listenConfigWithWatch() {
	var f func()
	f = func() {
		util.GoWithRecover(func() {
			r.listenConfig(20)
		}, func(i interface{}) {
			time.Sleep(time.Second * 3)
			f()
		})
	}
	f()
	var watch func()
	watch = func() {
		if r.ctx.Err() != nil {
			return
		}
		ctx, cancel := context.WithCancel(r.ctx)
		go func() {
			<-r.refresh
			cancel()
		}()
		for {
			r.watchConfig(ctx)
			if ctx.Err() != nil {
				break
			}
		}
		watch()
	}
	watch()
}

func sleep() {
	time.Sleep(10 * time.Second)
}

func (r *Rule) watchConfig(ctx context.Context) {
	subList := sc.Discovery().SubscribeAppID()
	count := len(subList)
	if count == 0 {
		sleep()
		return
	}
	args := make([]string, count*2)
	for i := 0; i < count; i++ {
		args[i] = fmt.Sprintf(RouterConfigKey, subList[i])
		updateAt := "$"
		args[i+count] = updateAt
		up, ok := r.getUpdateTime(args[i])
		if ok {
			if up == 0 {
				continue
			}
			updateAt = strconv.FormatInt(up, 10)
		} else {
			lastUpdateTime, err := rediscluster.Int64(sc.Redis().Do("HGET", args[i], RouterConfigUpdateField))
			if err != nil {
				if err == rediscluster.ErrNil {
					r.deleteEntity(subList[i])
					r.setUpdateTime(args[i], 0)
					continue
				}
				if errors.Is(err, redis.ErrClosed) {
					continue
				}
				cilog.LogErrorw(cilog.LogNameSidecar, "Pull traffic pull last update time error", err)
				continue
			}
			updateAt = strconv.FormatInt(lastUpdateTime, 10)
			uniqueID := r.getUniqueIDFromKey(args[i])
			conf := r.pullRouterConfig(uniqueID)
			if conf == nil {
				continue
			}
			r.ConfigRule(uniqueID, conf)
		}
		args[i+count] = updateAt
	}

	res, err := sc.Redis().WConfig(ctx, &redis.WConfigArgs{
		Configs:    args,
		StreamKey:  "watch_config:msp:service_router",
		ConsumerID: sc.Conf().Opts.MiscroServiceInfo.Hostname,
		Block:      time.Duration(21) * time.Second,
	})
	if err != nil {
		if errors.Is(err, rediscluster.ErrNil) || errors.Is(err, context.Canceled) {
			return
		}
		sleep()
		if errors.Is(err, redis.ErrClosed) {
			return
		}
		cilog.LogWarnf(cilog.LogNameSidecar, "Failed to listen to traffic pull configuration from redis proxy err: %s", err.Error())
		return
	}

	if len(res.Values) == 0 || res.ConfigID == "" {
		if res.ConfigID != "" {
			uniqueID := r.getUniqueIDFromKey(res.ConfigID)
			conf := r.pullRouterConfig(uniqueID)
			if conf == nil {
				return
			}
			r.ConfigRule(uniqueID, conf)
		}
		return
	}
	var lastUpdateTime int64
	if lastUpdateTimeStr, ok := res.Values[RouterConfigUpdateField]; ok {
		lastUpdateTime, err = strconv.ParseInt(lastUpdateTimeStr, 10, 64)
		if err == nil {
			r.setUpdateTime(res.ConfigID, lastUpdateTime)
		} else {
			cilog.LogWarnf(cilog.LogNameSidecar, "Sensing configuration change key %s Failed to parse the last update time of the configuration: %s", res.ConfigID, lastUpdateTimeStr)
		}
	}
	if data, ok := res.Values[RouterConfigField]; ok {
		uniqueID := r.getUniqueIDFromKey(res.ConfigID)
		cilog.LogInfof(cilog.LogNameSidecar, "Sensing that the router configuration has changed, the configuration belongs to uniqueID: %s The last update time of the configuration: %d", uniqueID, lastUpdateTime)
		r.ConfigRuleInByte(uniqueID, []byte(data), lastUpdateTime)
	} else {
		cilog.LogInfow(cilog.LogNameSidecar, "Error parsing the configuration, no config field found, please check if the configuration is correct")
	}
}

func (r *Rule) setUpdateTime(appid string, update int64) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.lastUpdateTimes[appid] = update
}

func (r *Rule) getUpdateTime(appid string) (update int64, ok bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	update, ok = r.lastUpdateTimes[appid]
	return
}

func (r *Rule) BatchGetRouterConfig() {
	subList := sc.Discovery().SubscribeAppID()
	for _, uniqueID := range subList {
		if uniqueID == "" {
			continue
		}
		conf := r.pullRouterConfig(uniqueID)
		if conf == nil {
			continue
		}
		r.ConfigRule(uniqueID, conf)
	}
}

func (r *Rule) getUniqueIDFromKey(key string) (uniqueID string) {
	return strings.TrimPrefix(key, RouterConfigKeyPrefix)
}

func (r *Rule) pullRouterConfig(uniqueID string) (conf *Config) {
	key := fmt.Sprintf(RouterConfigKey, uniqueID)
	lastUpdateTime, err := rediscluster.Int64(sc.Redis().Do("HGET", key, RouterConfigUpdateField))
	if err != nil {
		if err == rediscluster.ErrNil {
			r.deleteEntity(uniqueID)
			return
		}
		if errors.Is(err, redis.ErrClosed) {
			return
		}
		cilog.LogErrorw(cilog.LogNameSidecar, "Pull traffic pull last update time error", err)
		return
	}
	r.setUpdateTime(key, lastUpdateTime)
	if r.isErrorConfig(uniqueID, lastUpdateTime) || r.LastUpdateTime(uniqueID) >= lastUpdateTime {
		return
	}
	data, err := rediscluster.Bytes(sc.Redis().Do("HGET", key, RouterConfigField))
	if err != nil {
		if err == rediscluster.ErrNil || errors.Is(err, redis.ErrClosed) {
			return
		}
		cilog.LogErrorw(cilog.LogNameSidecar, "Pull traffic pull configuration error", err)
		return
	}
	err = json.Unmarshal(data, &conf)
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "json parsing traffic pulling configuration error", err)
		r.addErrorConfig(uniqueID, lastUpdateTime)
		conf = nil
		return
	}
	cilog.LogInfof(cilog.LogNameSidecar, "Induction flow configuration update:%s", string(data))
	return conf
}

func (r *Rule) ConfigRuleInByte(uniqueID string, data []byte, lastUpdateTime int64) {
	conf := new(Config)
	err := json.Unmarshal(data, &conf)
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "json parsing traffic pulling configuration error", err)
		r.addErrorConfig(uniqueID, lastUpdateTime)
		conf = nil
		return
	}
	r.ConfigRule(uniqueID, conf)
}

func (r *Rule) ConfigRule(uniqueID string, routerConfig *Config) {
	r.lock.RLock()
	_, ok := r.ruleList[uniqueID]
	r.lock.RUnlock()
	if ok {
		r.ReloadEntityRule(uniqueID, routerConfig)
		return
	}
	r.createEntityRule(uniqueID, routerConfig, nil)
}

func (r *Rule) Has(uniqueID string) bool {
	r.lock.RLock()
	defer r.lock.RUnlock()

	_, ok := r.ruleList[uniqueID]
	return ok
}

func (r *Rule) LastUpdateTime(uniqueID string) int64 {
	r.lock.RLock()
	defer r.lock.RUnlock()

	last, ok := r.ruleList[uniqueID]
	if !ok {
		return -1
	}
	return last.lastUpdateTime
}

func (r *Rule) ReloadEntityRule(uniqueID string, routers *Config) {
	r.lock.Lock()
	delete(r.ruleList, uniqueID)
	r.lock.Unlock()
	r.createEntityRule(uniqueID, routers, nil)
}

func (r *Rule) updateEntityRuleByDiscovery(uniqueID string, instanceList map[string][]*sesdk.Instance) {
	r.lock.Lock()
	conf := r.ruleList[uniqueID].conf
	delete(r.ruleList, uniqueID)
	r.lock.Unlock()
	r.createEntityRule(uniqueID, conf, instanceList)
}

func (r *Rule) createEntityRule(uniqueID string, routers *Config, instanceList map[string][]*sesdk.Instance) {
	if instanceList == nil {
		instanceList = sc.Discovery().AppIDInstances(uniqueID)
	}

	entityList := []*RuleEntity{}
	for k := range routers.Routers {
		if len(routers.Routers[k].Routes) == 0 && len(routers.Routers[k].Mirror) == 0 {
			cilog.LogError(cilog.LogNameSidecar, "Traffic rule configuration error："+string(util.JSONEncode(routers.Routers[k])))
			continue
		}
		match, err := NewMatches(routers.Routers[k].Match)
		if err != nil {
			cilog.LogErrorw(cilog.LogNameSidecar, "Traffic matching rule configuration error："+string(util.JSONEncode(routers.Routers[k].Match)), err)
			continue
		}
		entity := &RuleEntity{
			Match: match,
		}
		if len(routers.Routers[k].Routes) > 0 {
			entity.Distribution = NewDistribution()
			entity.Distribution.CreateRule(instanceList, routers.Routers[k].Routes)
			cilog.LogInfof(cilog.LogNameSidecar, "Create %s traffic pull rule: %s", uniqueID, util.JSONEncode(routers.Routers[k].Routes))
		}
		if len(routers.Routers[k].Mirror) > 0 {
			entity.Mirror = NewShadowRequest()
			entity.Mirror.SettingRule(instanceList, routers.Routers[k].Mirror)
			if len(entity.Mirror.TargetList) == 0 {
				entity.Mirror = nil
			} else {
				r.onceInitShadow.Do(r.initMirror)
				cilog.LogInfof(cilog.LogNameSidecar, "Create %s traffic mirroring rule: %s", uniqueID, util.JSONEncode(routers.Routers[k].Mirror))
			}
		}
		if entity.Distribution != nil || entity.Mirror != nil {
			entityList = append(entityList, entity)
		}
	}
	r.lock.Lock()
	r.ruleList[uniqueID] = UniqueIDRule{
		entity:         entityList,
		conf:           routers,
		lastUpdateTime: routers.Timestamp,
	}
	r.lock.Unlock()
}

func (r *Rule) deleteEntity(uniqueID string) {
	r.deleteErrorConfig(uniqueID)

	if !r.Has(uniqueID) {
		return
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	cilog.LogInfow(cilog.LogNameSidecar, "Delete traffic configuration rules："+uniqueID)
	delete(r.ruleList, uniqueID)
}

func (r *Rule) addErrorConfig(uniqueID string, lastUpdateTime int64) {
	r.errorConfigList.Store(uniqueID, lastUpdateTime)
}

func (r *Rule) isErrorConfig(uniqueID string, lastUpdateTime int64) bool {
	if last, ok := r.errorConfigList.Load(uniqueID); ok {
		if lastUpdateTime <= last.(int64) {
			return true
		}
		r.deleteErrorConfig(uniqueID)
	}
	return false
}

func (r *Rule) deleteErrorConfig(uniqueID string) {
	r.errorConfigList.Delete(uniqueID)
}

func (r *Rule) sendShadowRequest() {
	wg := sync.WaitGroup{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() {
				if err := recover(); err != nil {
					cilog.LogErrorw(cilog.LogNameSidecar, "Traffic mirroring asynchronous coroutine error interception：", fmt.Errorf("%v", err))
				}
			}()

			for req := range r.shadowRequestQueue {
				r.sendReq(req)
			}
		}(i)
	}
	wg.Wait()
}

func (r *Rule) sendReq(req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), time.Second*5)
	defer cancel()
	resp, err := r.httpClient.Transport.RoundTrip(req.WithContext(ctx))
	// resp, err := r.httpClient.Transport.RoundTrip(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	if resp.ContentLength < 65536 {
		_, _ = io.Copy(ioutil.Discard, resp.Body)
	}
}
