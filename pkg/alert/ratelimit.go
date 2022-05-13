package alert

import (
	"fmt"
	"time"

	"github.com/openmsp/sidecar/pkg/errno"
	"github.com/openmsp/sidecar/pkg/event"

	"github.com/openmsp/cilog"
)

const (
	RateLimitTypeService = 1
	RateLimitTypeBBR     = 2
	RateLimitTypeCodel   = 3
	RateLimitService     = "service distributed traffic limiting"
	RateLimitBBR         = "BBR current limiting"
	RateLimitCodel       = "CODEL current limiting"
)

type rateLimitAlertMsg struct {
	ServiceName string `json:"service_name"`
	UniqueID    string `json:"unique_id"`
	Hostname    string `json:"hostname"`
	Type        uint8  `json:"type"`
	EventTime   int64  `json:"event_time"`
}

func RateLimitAlert(uniqueID, hostname, serviceName string, err *errno.SCError) {
	var limitType uint8
	switch err.State {
	case errno.ErrLimiter.State:
		limitType = 1
	case errno.ErrBBRLimiter.State:
		limitType = 2
	case errno.ErrCodelLimiter.State:
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
	event.Client().Report(event.EventTypeRateLimit, rs)
}

func recordRateLimitWarning(params rateLimitAlertMsg) {
	var eventMsg string
	switch {
	case params.Type == RateLimitTypeService:
		eventMsg = RateLimitService
	case params.Type == RateLimitTypeBBR:
		eventMsg = RateLimitBBR
	case params.Type == RateLimitTypeCodel:
		eventMsg = RateLimitCodel
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
