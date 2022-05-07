package event

import (
	"encoding/json"
	"errors"
	"github.com/openmsp/sidecar/pkg/autoredis"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/openmsp/cilog"
)

const (
	MSP_EVENT             = "msp:event_msg"
	EVENT_TYPE_FUSE       = "fuse"
	EVENT_TYPE_RATE_LIMIT = "ratelimit"
	EVENT_TYPE_HEARTBEAT  = "heartbeat"
	EVENT_TYPE_RESOURCE   = "resource"
)

type EventReport struct {
	redisCluster autoredis.AutoClient
}

type MSPEvent struct {
	EventType string      `json:"event_type"` // fuse, ratelimit, heartbeat，resource
	EventTime int64       `json:"event_time"`
	EventBody interface{} `json:"event_body"`
}

var _eventReport *EventReport

func InitEventClient(rc autoredis.AutoClient) *EventReport {
	if _eventReport == nil {
		_eventReport = &EventReport{redisCluster: rc}
	}
	return _eventReport
}

func Client() *EventReport {
	return _eventReport
}

func (ev *EventReport) Report(evType string, eventBody interface{}) {
	msg := MSPEvent{
		EventType: evType,
		EventTime: time.Now().UnixNano() / 1e6,
		EventBody: eventBody,
	}
	b, err := json.Marshal(msg)
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "event report json.Marshal err", err)
		return
	}
	_, err = ev.redisCluster.Do("LPUSH", MSP_EVENT, b)

	if err != nil && !errors.Is(err, redis.ErrClosed) {
		cilog.LogErrorw(cilog.LogNameSidecar, "heartbeat report redis rpush", err)
	}
	return
}
