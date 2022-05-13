package fuse

import (
	"fmt"
	"time"

	"github.com/openmsp/sidecar/utils"

	"github.com/openmsp/cilog"
	"github.com/sony/gobreaker"
)

const (
	FuseOn       = 1
	FuseOff      = 2
	FuseStatus1  = "continuous errors reach the threshold for fusing"
	FuseStatus2  = "the error rate reaches a fixed percentage of fuses"
	FuseStatus3  = "The number of consecutive errors reaches the threshold or the error rate reaches the threshold"
	FuseStatus4  = "The number of consecutive errors reaches the threshold and the error rate simultaneously reaches the threshold"
	FuseAlertOn  = "the service fuse is enabled"
	FuseAlertOff = "the service is restored"

	TargetPath    = 1
	TargetHost    = 2
	TargetPathStr = "path"
	TargetHostStr = "host"
)

type fuseConfig struct {
	// The default value of the number of requests allowed to run when the
	// fuse is half turned on is 1.
	// If the request is successful, the circuit breaker is shut down
	MaxRequests uint32 `json:"max_requests"`
	// The clearing interval when the fuse is in the closed state.
	// The default value is 0.
	// If the fuse is always closed, the number of requests is not cleared
	Interval uint32 `json:"interval"`
	// error number threshold
	SerialErrorNumbers uint32 `json:"serial_error_numbers"`
	// When the fuse is in open state, how long after the trigger
	// is half open state, unit: s
	OpenTimeout uint32 `json:"open_timeout"`
	// request timeout duration unit s
	RequestTimeout uint32 `json:"request_timeout"`
	// error rate threshold unit：%
	ErrorPercent uint8 `json:"error_percent"`
	// The types of fuses are as follows:
	// 1 Fuses where the number of consecutive errors reaches the threshold.
	// 2 fuses where the error rate reaches a fixed percentage.
	// 3 fuses where the number of consecutive errors reaches the threshold or
	// the error rate reaches the threshold
	// 4 fuses where the number of consecutive errors reaches the threshold and
	// the error rate reaches the threshold at the same time
	RuleType uint8 `json:"rule_type"`
	// Whether to enable the rule. 1 Enable the rule. 2 Disable the rule
	IsOpen uint8 `json:"is_open"`
	// Path Interface level of a single instance.
	// Host Interface level of a single instance.
	// Default interface level
	Target string `json:"target"`
	// rule last updated
	Timestamp int64  `json:"timestamp"`
	UniqueID  string `json:"unique_id" binding:"required"`
}

type reportStats struct {
	FromUniqueID    string `json:"from_unique_id"`
	FromHostname    string `json:"from_hostname"`
	FromServiceName string `json:"from_service_name"`
	ToUniqueID      string `json:"to_unique_id"`
	ToHostname      string `json:"to_hostname"`
	Status          uint8  `json:"status"`
	FuseRuleType    uint8  `json:"fuse_rule_type"`
	EventTime       int64  `json:"event_time"`
}

func (fc *fuseConfig) getReadyToTripOne() func(counts gobreaker.Counts) bool {
	switch fc.RuleType {
	case 2:
		return fc.readyToTripTwo()
	case 3:
		return fc.readyToTripThree()
	case 4:
		return fc.readyToTripFour()
	default:
		return fc.readyToTripOne()
	}
}

func (fc *fuseConfig) getTarget() int8 {
	switch fc.Target {
	case TargetHostStr:
		return TargetHost
	case TargetPathStr:
		return TargetPath
	default:
		return TargetPath
	}
}

// strategy 1
// continuous errors reach the threshold for fusing
func (fc *fuseConfig) readyToTripOne() func(counts gobreaker.Counts) bool {
	return func(counts gobreaker.Counts) bool {
		return counts.ConsecutiveFailures >= fc.SerialErrorNumbers
	}
}

// strategy 2
// - the error rate reaches a fixed percentage of fuses
func (fc *fuseConfig) readyToTripTwo() func(counts gobreaker.Counts) bool {
	return func(counts gobreaker.Counts) bool {
		failureRatio := uint8(float64(counts.TotalFailures) / float64(counts.Requests) * 100)
		return counts.Requests >= 3 && failureRatio >= fc.ErrorPercent
	}
}

// strategy 3
// - The number of consecutive errors reaches the threshold or the error rate reaches the threshold
func (fc *fuseConfig) readyToTripThree() func(counts gobreaker.Counts) bool {
	return func(counts gobreaker.Counts) bool {
		failureRatio := uint8(float64(counts.TotalFailures) / float64(counts.Requests) * 100)
		return counts.ConsecutiveFailures >= fc.SerialErrorNumbers || failureRatio >= fc.ErrorPercent
	}
}

// strategy 4
// The number of consecutive errors reaches the threshold and the error rate simultaneously reaches the threshold
func (fc *fuseConfig) readyToTripFour() func(counts gobreaker.Counts) bool {
	return func(counts gobreaker.Counts) bool {
		failureRatio := uint8(float64(counts.TotalFailures) / float64(counts.Requests) * 100)
		return counts.ConsecutiveFailures >= fc.SerialErrorNumbers && failureRatio >= fc.ErrorPercent
	}
}

func (fc *fuseConfig) String() string {
	return string(utils.JSONEncode(fc))
}

// write to the fusing log
func recordFuseWarning(params reportStats) {
	var eventMsg, fuseMsg string
	if params.Status == FuseOn {
		eventMsg = FuseAlertOn
	} else if params.Status == FuseOff {
		eventMsg = FuseAlertOff
	}
	if params.Status == FuseOn {
		switch {
		case params.FuseRuleType == 1:
			fuseMsg = FuseStatus1
		case params.FuseRuleType == 2:
			fuseMsg = FuseStatus2
		case params.FuseRuleType == 3:
			fuseMsg = FuseStatus3
		case params.FuseRuleType == 4:
			fuseMsg = FuseStatus4
		}
	}
	textLog := fmt.Sprintf(`time of occurrence:%s;event:%s;from-service-name:%s;from-unique_id:%s;from-hostname:%s;to-unique_id:%s;to-hostname:%s;trigger rules:%s`,
		time.Unix(0, params.EventTime*int64(time.Millisecond)).Format("2006-01-02 15:04:05.000"),
		eventMsg,
		params.FromServiceName,
		params.FromUniqueID,
		params.FromHostname,
		params.ToUniqueID,
		params.ToHostname,
		fuseMsg,
	)
	if params.Status == FuseOn {
		cilog.LogError(cilog.LogNameSidecar, textLog)
	} else if params.Status == FuseOff {
		cilog.LogInfof(cilog.LogNameSidecar, textLog)
	}
}
