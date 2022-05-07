package fuse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/pkg/errno"
	"github.com/openmsp/sidecar/pkg/event"
	"github.com/openmsp/sidecar/pkg/out"
	"github.com/openmsp/sidecar/pkg/remoteconfer"
	"github.com/openmsp/sidecar/sc"
	"github.com/openmsp/sidecar/utils"

	"github.com/labstack/echo/v4"

	"github.com/sony/gobreaker"

	"github.com/openmsp/cilog"
)

type CircuitBreaker struct {
	// upstream target service：uniqueID
	upstream map[string]*upstreamBreaker
	// default setting
	defaultFC *fuseConfig
	// configure the channel for receiving data
	configChan <-chan []byte
	lock       sync.RWMutex
	// The control plane delivers the configured unique ID list
	controlPlaneUniqueID []string
}

type upstreamBreaker struct {
	sync.RWMutex
	requestTimeout time.Duration
	config         *fuseConfig
	// 1 Path indicates the interface granularity,
	// and 2 host indicates the single instance level
	target int8
	// hostname: *gobreaker.TwoStepCircuitBreaker
	hostname map[string]*gobreaker.TwoStepCircuitBreaker
}

func (f *upstreamBreaker) IsPathTarget() bool {
	return f.target == TargetPath
}

func (f *CircuitBreaker) ExecuteCircuitBreakerNetHTTPMiddle(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		app := c.Get(confer.AppInfoKey).(*confer.RemoteApp)
		done, timeout, err := f.ExecuteCircuitBreaker(app.AppID, app.Hostname, app.Path)
		if err != nil {
			return out.HTTPJsonResponseError(c, app, err)
		}
		app.Timeout = timeout

		err = next(c)
		done(err == nil && c.Response().Status < http.StatusBadRequest && c.Response().Status >= http.StatusOK)
		return err
	}
}

func (f *CircuitBreaker) ExecuteCircuitBreaker(uniqueID, hostname, path string) (func(bool), time.Duration, error) {
	if uniqueID == "" || hostname == "" {
		return nil, 0, errno.FuseError.Add(fmt.Sprintf("uniqueId:%s or hostname:%s null", uniqueID, hostname))
	}
	breaker, requestTimeout, ok := f.getGoBreaker(uniqueID, hostname, path)
	if !ok {
		breaker, requestTimeout = f.registerCircuitBreaker(uniqueID, hostname, path)
	}
	done, err := breaker.Allow()
	if err != nil {
		return nil, 0, errno.FuseError.Add(err.Error())
	}
	return done, requestTimeout, nil
}

func GetCircuitBreaker() *CircuitBreaker {
	fn := &CircuitBreaker{
		upstream: make(map[string]*upstreamBreaker),
		defaultFC: &fuseConfig{
			UniqueID:           confer.Global().Opts.MiscroServiceInfo.UniqueID,
			MaxRequests:        confer.Global().Opts.Fuse.MaxRequest,
			Interval:           confer.Global().Opts.Fuse.Interval,
			ErrorPercent:       confer.Global().Opts.Fuse.ErrorPercent,
			SerialErrorNumbers: confer.Global().Opts.Fuse.SerialErrorNumbers,
			OpenTimeout:        confer.Global().Opts.Fuse.Timeout,
			RequestTimeout:     confer.Global().Opts.Fuse.RequestTimeout,
			RuleType:           confer.Global().Opts.Fuse.RuleType,
			Target:             confer.Global().Opts.Fuse.Target,
			IsOpen:             1,
			Timestamp:          utils.TimeMs(),
		},
	}
	fn.configChan = sc.Sc().RegisterConfigWatcher(remoteconfer.MspServiceRuleFuseKey, func(key, value []byte) bool {
		return string(key) == remoteconfer.MspServiceRuleFuseKey
	}, 1)
	fn.listenConfig()
	return fn
}

func (f *CircuitBreaker) registerCircuitBreaker(uniqueID, hostname, path string) (*gobreaker.TwoStepCircuitBreaker, time.Duration) {
	f.lock.RLock()
	upBreaker, ok := f.upstream[uniqueID]
	f.lock.RUnlock()

	if ok {
		if upBreaker.IsPathTarget() {
			hostname += path
		}
		upBreaker.RLock()
		breaker, ok := upBreaker.hostname[hostname]
		upBreaker.RUnlock()
		if ok {
			return breaker, upBreaker.requestTimeout
		}
	} else {
		upBreaker = &upstreamBreaker{}
		upBreaker.config = f.defaultFC
		upBreaker.target = TargetPath
		upBreaker.requestTimeout = time.Duration(f.defaultFC.RequestTimeout) * time.Second
		upBreaker.hostname = make(map[string]*gobreaker.TwoStepCircuitBreaker)
		f.lock.Lock()
		f.upstream[uniqueID] = upBreaker
		f.lock.Unlock()
		if upBreaker.IsPathTarget() {
			hostname += path
		}
	}

	upBreaker.Lock()
	defer upBreaker.Unlock()

	var st gobreaker.Settings
	if upBreaker.config.IsOpen == FUSE_ON {
		st = gobreaker.Settings{
			Name:          f.joinBreakerName(uniqueID, hostname),
			MaxRequests:   upBreaker.config.MaxRequests,
			Interval:      time.Second * time.Duration(upBreaker.config.Interval),
			Timeout:       time.Second * time.Duration(upBreaker.config.OpenTimeout),
			ReadyToTrip:   upBreaker.config.getReadyToTripOne(),
			OnStateChange: f.onStateChange,
		}
	} else {
		st = gobreaker.Settings{
			Name:          f.joinBreakerName(uniqueID, hostname),
			MaxRequests:   f.defaultFC.MaxRequests,
			Interval:      time.Second * time.Duration(f.defaultFC.Interval),
			Timeout:       time.Second * time.Duration(f.defaultFC.OpenTimeout),
			ReadyToTrip:   f.defaultFC.getReadyToTripOne(),
			OnStateChange: f.onStateChange,
		}
	}
	breaker := gobreaker.NewTwoStepCircuitBreaker(st)
	upBreaker.hostname[hostname] = breaker
	return breaker, upBreaker.requestTimeout
}

func (f *CircuitBreaker) updateCircuitBreaker(config *fuseConfig) {
	toUniqueID := config.UniqueID
	f.lock.RLock()
	upBreaker, ok := f.upstream[toUniqueID]
	if ok && upBreaker.config.Timestamp >= config.Timestamp {
		f.lock.RUnlock()
		return
	}
	f.lock.RUnlock()
	f.checkFc(config)
	if !ok {
		upBreaker = &upstreamBreaker{}
		upBreaker.config = config
		upBreaker.target = config.getTarget()
		upBreaker.requestTimeout = time.Duration(config.RequestTimeout) * time.Second
		upBreaker.hostname = make(map[string]*gobreaker.TwoStepCircuitBreaker)
		f.lock.Lock()
		f.upstream[toUniqueID] = upBreaker
		f.lock.Unlock()
		cilog.LogInfof(cilog.LogNameSidecar, "added fuse configuration：%s : %s", toUniqueID, config)
		return
	}

	upBreaker.Lock()
	defer upBreaker.Unlock()
	upBreaker.requestTimeout = time.Duration(config.RequestTimeout) * time.Second
	upBreaker.config = config
	upBreaker.hostname = make(map[string]*gobreaker.TwoStepCircuitBreaker)
	cilog.LogInfof(cilog.LogNameSidecar, "update fuse configuration：%s : %s", toUniqueID, config)
}

func (f *CircuitBreaker) clearBreaker(conf map[string]*fuseConfig) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if len(conf) == 0 {
		for _, uniqueID := range f.controlPlaneUniqueID {
			delete(f.upstream, uniqueID)
		}
		f.controlPlaneUniqueID = f.controlPlaneUniqueID[:0]
		cilog.LogWarnw(cilog.LogNameSidecar, "Clear all circuit breakers configured on the control plane and use the default circuit breaker rules")
	} else {
		newSlice := make([]string, len(conf))
		for _, uniqueID := range f.controlPlaneUniqueID {
			if _, ok := conf[uniqueID]; !ok {
				delete(f.upstream, uniqueID)
				cilog.LogInfof(cilog.LogNameSidecar, "Remove %s circuit breaker, use default circuit breaker", uniqueID)
			}
		}
		i := 0
		for uniqueID := range conf {
			newSlice[i] = uniqueID
			i++
		}
		f.controlPlaneUniqueID = newSlice
	}
}

func (f *CircuitBreaker) joinBreakerName(uniqueID, hostname string) string {
	buf := bytes.NewBufferString(uniqueID)
	buf.WriteString("|")
	buf.WriteString(hostname)
	return buf.String()
}

func (f *CircuitBreaker) parseBreakerName(name string) (string, string) {
	s := strings.Split(name, "|")
	return s[0], s[1]
}

func (f *CircuitBreaker) checkFc(fc *fuseConfig) {
	if fc.RuleType < 1 || fc.RuleType > 4 {
		fc.RuleType = 1
	}
	if fc.ErrorPercent < 1 || fc.ErrorPercent > 100 {
		fc.ErrorPercent = 1
	}
	if fc.SerialErrorNumbers < 1 {
		fc.SerialErrorNumbers = 1
	}
	if fc.RequestTimeout < 1 {
		fc.RequestTimeout = 1
	}
}

func (f *CircuitBreaker) onStateChange(name string, from gobreaker.State, to gobreaker.State) {
	uniqueID, hostname := f.parseBreakerName(name)
	cilog.LogInfof(cilog.LogNameSidecar, "fusing status changed, toUniqueID: %s, toHostname: %s, from: %s  to: %s", uniqueID, hostname, from.String(), to.String())
	if (from == gobreaker.StateHalfOpen && to == gobreaker.StateOpen) || (from == gobreaker.StateOpen && to == gobreaker.StateHalfOpen) {
		return
	}

	rs := reportStats{
		FromUniqueID:    confer.Global().Opts.MiscroServiceInfo.UniqueID,
		FromHostname:    confer.Global().Opts.MiscroServiceInfo.Hostname,
		FromServiceName: confer.Global().Opts.MiscroServiceInfo.ServiceName,
		ToUniqueID:      uniqueID,
		ToHostname:      hostname,
		Status:          0,
		EventTime:       utils.TimeMs(),
	}

	f.lock.RLock()
	if fc, ok := f.upstream[uniqueID]; ok {
		rs.FuseRuleType = fc.config.RuleType
	} else {
		rs.FuseRuleType = f.defaultFC.RuleType
	}
	f.lock.RUnlock()
	if to == gobreaker.StateOpen {
		rs.Status = 1
	} else {
		rs.Status = 2
	}
	recordFuseWarning(rs)
	event.Client().Report(event.EVENT_TYPE_FUSE, rs)
}

func (f *CircuitBreaker) getGoBreaker(uniqueID, hostname, path string) (*gobreaker.TwoStepCircuitBreaker, time.Duration, bool) {
	f.lock.RLock()
	ub, ok := f.upstream[uniqueID]
	f.lock.RUnlock()
	if !ok {
		return nil, 0, false
	}
	ub.RLock()
	defer ub.RUnlock()
	if ub.IsPathTarget() {
		hostname += path
	}
	hostBreaker, ok := ub.hostname[hostname]
	if !ok {
		return nil, 0, false
	}
	return hostBreaker, ub.requestTimeout, true
}

func (f *CircuitBreaker) listenConfig() {
	utils.GoWithRecover(func() {
		f.listenForConfigurationChanges()
	}, func(_ interface{}) {
		time.Sleep(time.Second)
		f.listenConfig()
	})
}

func (f *CircuitBreaker) listenForConfigurationChanges() {
	for data := range f.configChan {
		if len(data) == 0 {
			if len(f.controlPlaneUniqueID) > 0 {
				f.clearBreaker(nil)
			}
			continue
		}
		var redisConfig map[string]*fuseConfig
		err := json.Unmarshal(data, &redisConfig)
		if err != nil {
			cilog.LogErrorw(cilog.LogNameSidecar, "fuse configuration resolution error", err)
			continue
		}
		f.clearBreaker(redisConfig)
		for toUniqueID := range redisConfig {
			redisConfig[toUniqueID].UniqueID = toUniqueID
			if len(toUniqueID) == 0 {
				cilog.LogErrorw(cilog.LogNameSidecar, "The fuse is incorrectly configured and unique ID does not exist：", err)
				continue
			}
			f.updateCircuitBreaker(redisConfig[toUniqueID])
		}
	}
}
