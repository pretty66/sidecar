package cpu

import (
	"sync/atomic"
	"time"

	"github.com/openmsp/cilog"
)

const (
	interval = time.Millisecond * 500
)

var (
	stats CPU
	usage uint64
)

// CPU is cpu stat usage.
type CPU interface {
	Usage() (u uint64, e error)
	Info() Info
}

func StartInit() (err error) {
	stats, err = newCgroupCPU()

	if err != nil {
		// fmt.Printf("cgroup cpu init failed(%v),switch to psutil cpu\n", err)
		cilog.LogErrorw(cilog.LogNameSidecar, "cgroup cpu init failed(),switch to psutil cpu err ", err)
		stats, err = newPsutilCPU(interval)
		if err != nil {
			// panic(fmt.Sprintf("cgroup cpu init failed!err:=%v", err))
			cilog.LogErrorw(cilog.LogNameSidecar, "cgroup cpu init failed!err:=%v", err)
		}
	}

	if err != nil {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			<-ticker.C
			u, err := stats.Usage()
			if err == nil && u != 0 {
				atomic.StoreUint64(&usage, u)
			}
		}
	}()
	return
}

// Stat cpu stat.
type Stat struct {
	Usage uint64 // cpu use ratio.
}

// Info cpu info.
type Info struct {
	Frequency uint64
	Quota     float64
}

// ReadStat read cpu stat.
func ReadStat(stat *Stat) {
	stat.Usage = atomic.LoadUint64(&usage)
}

// GetInfo get cpu info.
func GetInfo() Info {
	return stats.Info()
}
