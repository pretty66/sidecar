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
	MspEvent           = "msp:event_msg"
	EventTypeFuse      = "fuse"
	EventTypeRateLimit = "ratelimit"
	EventTypeHeartbeat = "heartbeat"
	EventTypeResource  = "resource"
)

type Report struct {
	redisCluster autoredis.AutoClient
}

type MSPEvent struct {
	EventType string      `json:"event_type"` // fuse, ratelimit, heartbeat，resource
	EventTime int64       `json:"event_time"`
	EventBody interface{} `json:"event_body"`
}

var _eventReport *Report

func InitEventClient(rc autoredis.AutoClient) *Report {
	if _eventReport == nil {
		_eventReport = &Report{redisCluster: rc}
	}
	return _eventReport
}

func Client() *Report {
	return _eventReport
}

func (ev *Report) Report(evType string, eventBody interface{}) {
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
	_, err = ev.redisCluster.Do("LPUSH", MspEvent, b)

	if err != nil && !errors.Is(err, redis.ErrClosed) {
		cilog.LogErrorw(cilog.LogNameSidecar, "heartbeat report redis rpush", err)
	}
	return
}
