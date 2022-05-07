package alert

import (
	"fmt"
	"time"

	"github.com/openmsp/sidecar/pkg/errno"
	"github.com/openmsp/sidecar/pkg/event"

	"github.com/openmsp/cilog"
)

const (
	RATE_LIMIT_TYPE_SERVICE = 1
	RATE_LIMIT_TYPE_BBR     = 2
	RATE_LIMIT_TYPE_CODEL   = 3
	RATE_LIMIT_SERVICE      = "service distributed traffic limiting"
	RATE_LIMIT_BBR          = "BBR current limiting"
	RATE_LIMIT_CODEL        = "CODEL current limiting"
)

type rateLimitAlertMsg struct {
	ServiceName string `json:"service_name"`
	UniqueID    string `json:"unique_id"`
	Hostname    string `json:"hostname"`
	Type        uint8  `json:"type"`
	EventTime   int64  `json:"event_time"`
}

func RateLimitAlert(uniqueID, hostname, serviceName string, err *errno.Errno) {
	var limitType uint8
	switch err.State {
	case errno.LimiterError.State:
		limitType = 1
	case errno.BbrLimiterError.State:
		limitType = 2
	case errno.CodelLimiterError.State:
		limitType = 3
	default:
		return
	}
	rs := rateLimitAlertMsg{
		EventTime:   time.Now().UnixNano() / 1e6,
		UniqueID:    uniqueID,
		ServiceName: serviceName,
		Hostname:    hostname,
		Type:        limitType,
	}
	recordRateLimitWarning(rs)
	event.Client().Report(event.EVENT_TYPE_RATE_LIMIT, rs)
}

func recordRateLimitWarning(params rateLimitAlertMsg) {
	var eventMsg string
	switch {
	case params.Type == RATE_LIMIT_TYPE_SERVICE:
		eventMsg = RATE_LIMIT_SERVICE
	case params.Type == RATE_LIMIT_TYPE_BBR:
		eventMsg = RATE_LIMIT_BBR
	case params.Type == RATE_LIMIT_TYPE_CODEL:
		eventMsg = RATE_LIMIT_CODEL
	}
	textLog := fmt.Sprintf(`Time of occurrence:%s;event:%s;service name:%s;unique_id:%s;hostname:%s;trigger rules:%s`,
		time.Unix(0, params.EventTime*int64(time.Millisecond)).Format("2006-01-02 15:04:05.000"),
		"service current limiting",
		params.ServiceName,
		params.UniqueID,
		params.Hostname,
		eventMsg,
	)
	cilog.LogError(cilog.LogNameDefault, textLog)
}
