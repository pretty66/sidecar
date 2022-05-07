package bbr

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/openmsp/sidecar/pkg/container/group"
	"github.com/openmsp/sidecar/pkg/errno"
	"github.com/openmsp/sidecar/pkg/out"
	"github.com/openmsp/sidecar/pkg/ratelimit"
	"github.com/openmsp/sidecar/pkg/stat/metric"

	"github.com/labstack/echo/v4"

	"github.com/openmsp/cilog"

	cpustat "github.com/openmsp/sidecar/pkg/stat/sys/cpu"
)

var (
	cpu         int64
	decay       = 0.95
	initTime    = time.Now()
	defaultConf = &Config{
		Window:       time.Second * 10,
		WinBucket:    100,
		CPUThreshold: 800,
	}
)

type cpuGetter func() int64

// cpu = cpuᵗ⁻¹ * decay + cpuᵗ * (1 - decay)
func cpuproc() {
	ticker := time.NewTicker(time.Millisecond * 250)
	defer func() {
		ticker.Stop()
		if err := recover(); err != nil {
			// log.Error("rate.limit.cpuproc() err(%+v)", err)
			cilog.LogErrorw(cilog.LogNameSidecar, "rate.limit.cpuproc() err ", err.(error))
			go cpuproc()
		}
	}()

	// EMA algorithm: https://blog.csdn.net/m0_38106113/article/details/81542863
	for range ticker.C {
		stat := &cpustat.Stat{}
		cpustat.ReadStat(stat)
		prevCPU := atomic.LoadInt64(&cpu)
		curCPU := int64(float64(prevCPU)*decay + float64(stat.Usage)*(1.0-decay))
		atomic.StoreInt64(&cpu, curCPU)
	}
}

// Stat contains the metrics's snapshot of bbr.
type Stat struct {
	CPU         int64
	InFlight    int64
	MaxInFlight int64
	MinRt       int64
	MaxPass     int64
}

// BBR implements bbr-like limiter.
// It is inspired by sentinel.
// https://github.com/alibaba/Sentinel/wiki/%E7%B3%BB%E7%BB%9F%E8%87%AA%E9%80%82%E5%BA%94%E9%99%90%E6%B5%81
type BBR struct {
	cpu             cpuGetter
	passStat        metric.RollingCounter
	rtStat          metric.RollingCounter
	inFlight        int64
	winBucketPerSec int64
	bucketDuration  time.Duration
	winSize         int
	conf            *Config
	prevDrop        atomic.Value
	maxPASSCache    atomic.Value
	minRtCache      atomic.Value
}

// CounterCache is used to cache maxPASS and minRt result.
// Value of current bucket is not counted in real time.
// Cache time is equal to a bucket duration.
type CounterCache struct {
	val  int64
	time time.Time
}

// Config contains configs of bbr limiter.
type Config struct {
	Enabled      bool
	Window       time.Duration
	WinBucket    int
	Rule         string
	Debug        bool
	CPUThreshold int64
}

func (l *BBR) maxPASS() int64 {
	passCache := l.maxPASSCache.Load()
	if passCache != nil {
		ps := passCache.(*CounterCache)
		if l.timespan(ps.time) < 1 {
			return ps.val
		}
	}
	rawMaxPass := int64(l.passStat.Reduce(func(iterator metric.Iterator) float64 {
		result := 1.0
		for i := 1; iterator.Next() && i < l.conf.WinBucket; i++ {
			bucket := iterator.Bucket()
			count := 0.0
			for _, p := range bucket.Points {
				count += p
			}
			result = math.Max(result, count)
		}
		return result
	}))
	if rawMaxPass == 0 {
		rawMaxPass = 1
	}
	l.maxPASSCache.Store(&CounterCache{
		val:  rawMaxPass,
		time: time.Now(),
	})
	return rawMaxPass
}

func (l *BBR) timespan(lastTime time.Time) int {
	v := int(time.Since(lastTime) / l.bucketDuration)
	if v > -1 {
		return v
	}
	return l.winSize
}

func (l *BBR) minRT() int64 {
	rtCache := l.minRtCache.Load()
	if rtCache != nil {
		rc := rtCache.(*CounterCache)
		if l.timespan(rc.time) < 1 {
			return rc.val
		}
	}
	rawMinRT := int64(math.Ceil(l.rtStat.Reduce(func(iterator metric.Iterator) float64 {
		result := math.MaxFloat64
		for i := 1; iterator.Next() && i < l.conf.WinBucket; i++ {
			bucket := iterator.Bucket()
			if len(bucket.Points) == 0 {
				continue
			}
			total := 0.0
			for _, p := range bucket.Points {
				total += p
			}
			avg := total / float64(bucket.Count)
			result = math.Min(result, avg)
		}
		return result
	})))
	if rawMinRT <= 0 {
		rawMinRT = 1
	}
	l.minRtCache.Store(&CounterCache{
		val:  rawMinRT,
		time: time.Now(),
	})
	return rawMinRT
}

func (l *BBR) maxFlight() int64 {
	return int64(math.Floor(float64(l.maxPASS()*l.minRT()*l.winBucketPerSec)/1000.0 + 0.5))
}

func (l *BBR) shouldDrop() bool {
	if l.cpu() < l.conf.CPUThreshold {
		prevDrop, _ := l.prevDrop.Load().(time.Duration)
		if prevDrop == 0 {
			return false
		}
		if time.Since(initTime)-prevDrop <= time.Second {
			inFlight := atomic.LoadInt64(&l.inFlight)
			return inFlight > 1 && inFlight > l.maxFlight()
		}
		l.prevDrop.Store(time.Duration(0))
		return false
	}
	inFlight := atomic.LoadInt64(&l.inFlight)
	drop := inFlight > 1 && inFlight > l.maxFlight()
	if drop {
		prevDrop, _ := l.prevDrop.Load().(time.Duration)
		if prevDrop != 0 {
			return drop
		}
		l.prevDrop.Store(time.Since(initTime))
	}
	return drop
}

// Stat tasks a snapshot of the bbr limiter.
func (l *BBR) Stat() Stat {
	return Stat{
		CPU:         l.cpu(),
		InFlight:    atomic.LoadInt64(&l.inFlight),
		MinRt:       l.minRT(),
		MaxPass:     l.maxPASS(),
		MaxInFlight: l.maxFlight(),
	}
}

// Allow checks all inbound traffic.
// Once overload is detected, it raises ecode.LimitExceed error.
func (l *BBR) Allow(ctx context.Context, opts ...ratelimit.AllowOption) (func(info ratelimit.DoneInfo), error) {
	allowOpts := ratelimit.DefaultAllowOpts()
	for _, opt := range opts {
		opt.Apply(&allowOpts)
	}
	if l.shouldDrop() {
		return nil, errno.BbrLimiterError
	}
	atomic.AddInt64(&l.inFlight, 1)
	stime := time.Since(initTime)
	return func(do ratelimit.DoneInfo) {
		rt := int64((time.Since(initTime) - stime) / time.Millisecond)
		l.rtStat.Add(rt)
		atomic.AddInt64(&l.inFlight, -1)
		switch do.Op {
		case ratelimit.Success:
			l.passStat.Add(1)
			return
		default:
			return
		}
	}, nil
}

func newLimiter(conf *Config) ratelimit.Limiter {
	if conf == nil {
		conf = defaultConf
	}
	size := conf.WinBucket
	bucketDuration := conf.Window / time.Duration(conf.WinBucket)
	passStat := metric.NewRollingCounter(metric.RollingCounterOpts{Size: size, BucketDuration: bucketDuration})
	rtStat := metric.NewRollingCounter(metric.RollingCounterOpts{Size: size, BucketDuration: bucketDuration})
	cpu := func() int64 {
		return atomic.LoadInt64(&cpu)
	}
	limiter := &BBR{
		cpu:             cpu,
		conf:            conf,
		passStat:        passStat,
		rtStat:          rtStat,
		winBucketPerSec: int64(time.Second) / (int64(conf.Window) / int64(conf.WinBucket)),
		bucketDuration:  bucketDuration,
		winSize:         conf.WinBucket,
	}
	return limiter
}

// Group represents a class of BBRLimiter and forms a namespace in which
// units of BBRLimiter.
type Group struct {
	group *group.Group
}

// NewGroup new a limiter group container, if conf nil use default conf.
func NewGroup(conf *Config) (groupRet *Group, err error) {
	err = cpustat.StartInit()

	if err != nil {
		return
	}
	go cpuproc()

	if conf == nil {
		conf = defaultConf
	}
	group, err := group.NewGroup(func() interface{} {
		return newLimiter(conf)
	})
	if err != nil {
		return nil, err
	}

	groupRet = &Group{
		group: group,
	}
	return groupRet, nil
}

// Get get a limiter by a specified key, if limiter not exists then make a new one.
func (g *Group) Get(key string) ratelimit.Limiter {
	limiter := g.group.Get(key)
	return limiter.(ratelimit.Limiter)
}

func (l *BBR) HTTPBBRAllow(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		done, err := l.Allow(context.TODO())
		if err != nil {
			e, _ := err.(*errno.Errno)
			state, _ := json.Marshal(l.Stat())
			cilog.LogWarnf(cilog.LogNameSidecar, "bbr be triggered，bbr state：%s", string(state))
			return out.HTTPJsonRateLimitError(c.Response(), http.StatusTooManyRequests, e)
		}

		err = next(c)
		op := ratelimit.Ignore

		if c.Response().Status < 400 && c.Response().Status >= 200 {
			op = ratelimit.Success
		}
		done(ratelimit.DoneInfo{Op: op})
		return err
	}
}
