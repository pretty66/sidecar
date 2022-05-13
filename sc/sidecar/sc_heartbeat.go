package sidecar

import (
	"errors"
	"fmt"
	"github.com/openmsp/sidecar/pkg/event"
	"log"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/openmsp/cilog"
	"github.com/openmsp/sesdk"
	"github.com/openmsp/sesdk/discovery"
	"github.com/openmsp/sesdk/pkg/health"
)

const (
	HEART_BEAT_STATUS_ON  = 1
	HEART_BEAT_STATUS_OFF = 2
	HEART_BEAT_ON         = "Service online"
	HEART_BEAT_OFF        = "Service offline"
	StartSilenceTime      = 30
)

type heartbeatAlert struct {
	UniqueID    string `json:"unique_id"`
	Hostname    string `json:"hostname"`
	ServiceName string `json:"service_name"`
	Status      uint8  `json:"status"`
	EventTime   int64  `json:"event_time"`
}

var healthcheckStatus atomic.Value

func (tp *TrafficProxy) heartbeat(ins *sesdk.Instance) {
	if !tp.Confer.Opts.Heartbeat.Enable {
		return
	}
	log.Printf("Start the heartbeat, every %d seconds.", tp.Confer.Opts.Heartbeat.Gap)

	startUnixTime := time.Now().Unix()
	heartbeat := health.NewHealth(tp.Confer.Opts.Heartbeat.API, time.Duration(tp.Confer.Opts.Heartbeat.Timeout)*time.Second)

	is, out, err := heartbeat.GetHealth()

	healthcheckStatus.Store(is)
	if err != nil {
	} else if is {
		ins.Status = discovery.InstanceStatusReceive
		err = tp.Discovery.Set(ins)
		if err != nil {
			ins.Status = discovery.InstanceStatusNotReceive
		} else {
			startUnixTime = 0
			cilog.LogInfof(cilog.LogNameSidecar, "register success, output:%s", string(out))
		}
	}
	heartbeat.StartHealth(tp.OfflineCtx, time.Duration(tp.Confer.Opts.Heartbeat.Gap)*time.Second, func(count *health.Counts, is bool, out []byte, err error) {
		healthcheckStatus.Store(is)
		if count.ConsecutiveSuccesses >= tp.Confer.Opts.Heartbeat.ConsecutiveSuccesses && ins.Status == discovery.InstanceStatusNotReceive {
			startUnixTime = 0
			ins.Status = discovery.InstanceStatusReceive
			err = tp.Discovery.Set(ins)
			if err != nil {
				ins.Status = discovery.InstanceStatusNotReceive
				return
			}
			ha := heartbeatAlert{
				UniqueID:    tp.Confer.Opts.MiscroServiceInfo.UniqueID,
				Hostname:    tp.Confer.Opts.MiscroServiceInfo.Hostname,
				ServiceName: tp.Confer.Opts.MiscroServiceInfo.ServiceName,
				Status:      discovery.InstanceStatusReceive,
				EventTime:   time.Now().UnixNano() / 1e6,
			}

			recordHeartBeatWarning(ha)
			event.Client().Report(event.EVENT_TYPE_HEARTBEAT, ha)
			return
		}
		if count.ConsecutiveFailures >= tp.Confer.Opts.Heartbeat.ConsecutiveFailures && ins.Status == discovery.InstanceStatusReceive {
			ins.Status = discovery.InstanceStatusNotReceive
			err = tp.Discovery.Set(ins)
			if err != nil {
				ins.Status = discovery.InstanceStatusReceive
				return
			}
			ha := heartbeatAlert{
				UniqueID:    tp.Confer.Opts.MiscroServiceInfo.UniqueID,
				Hostname:    tp.Confer.Opts.MiscroServiceInfo.Hostname,
				ServiceName: tp.Confer.Opts.MiscroServiceInfo.ServiceName,
				Status:      discovery.InstanceStatusNotReceive,
				EventTime:   time.Now().UnixNano() / 1e6,
			}
			recordHeartBeatWarning(ha)
			event.Client().Report(event.EVENT_TYPE_HEARTBEAT, ha)
			return
		}
		if !is || err != nil {
			es := ""
			if err != nil {
				es = err.Error()
			}

			if startUnixTime == 0 || time.Now().Unix()-startUnixTime > StartSilenceTime {
				cilog.LogWarnf(cilog.LogNameSidecar, "Health check failed, error:%s, output:%v", es, string(out))
			}
		}
	})
}

func recordHeartBeatWarning(params heartbeatAlert) {
	var eventMsg string

	if params.Status == HEART_BEAT_STATUS_ON {
		eventMsg = HEART_BEAT_ON
	} else if params.Status == HEART_BEAT_STATUS_OFF {
		eventMsg = HEART_BEAT_OFF
	}

	textLog := fmt.Sprintf(`Time of occurrence::%s;event:%s;service name:%s;unique_id:%s;hostname: %s`,
		time.Unix(0, params.EventTime*int64(time.Millisecond)).Format("2006-01-02 15:04:05.000"),
		eventMsg,
		params.ServiceName,
		params.UniqueID,
		params.Hostname,
	)
	if params.Status == HEART_BEAT_STATUS_ON {
		cilog.LogInfof(cilog.LogNameSidecar, textLog)
	} else if params.Status == HEART_BEAT_STATUS_OFF {
		cilog.LogError(cilog.LogNameSidecar, textLog)
	}
}

func (tp *TrafficProxy) k8sheartbeat() {
	if !tp.Confer.Opts.Heartbeat.Enable {
		healthcheckStatus.Store(true)
	}
	mux := http.NewServeMux()
	mux.HandleFunc(tp.Confer.Opts.K8SHeartbeat.API, func(writer http.ResponseWriter, request *http.Request) {
		var status int
		if !healthcheckStatus.Load().(bool) {
			status = http.StatusInternalServerError
		} else {
			status = http.StatusOK
		}
		writer.WriteHeader(status)
	})
	err := http.ListenAndServe(tp.Confer.Opts.K8SHeartbeat.BindAddress, mux)
	if err != nil && !errors.Is(err, net.ErrClosed) {
		cilog.LogErrorw(cilog.LogNameSidecar, "k8s Failed to listen to the health check port", err)
	}
}
