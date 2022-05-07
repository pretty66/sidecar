package influxdb

import (
	"log"
	"math/rand"
	"testing"
	"time"

	"github.com/openmsp/cilog"
)

func TestLoggerControl(t *testing.T) {
	lc := NewLoggerControl(time.Millisecond * 1000)
	a := func() {
		var i int
		for i < 10000000 {
			select {
			case <-lc.timer.C:
				lc.Reset()
				cilog.LogWarnf(cilog.LogNameSidecar, "add point to send to influxdb failed name %d------%d", i, (time.Now().UnixNano()/int64(time.Millisecond))%10000)
			default:
				log.Printf("count %d \n", i)
			}
			time.Sleep(time.Millisecond * time.Duration(rand.Int()%1500))
			i++
		}
	}
	go a()
	// go a()
	select {}
}
