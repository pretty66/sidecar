package influxdb

import "time"

type LoggerControl struct {
	duration time.Duration
	timer    *time.Timer
}

func NewLoggerControl(duration time.Duration) *LoggerControl {
	return &LoggerControl{
		duration: duration,
		timer:    time.NewTimer(duration),
	}
}

func (l *LoggerControl) Reset() {
	l.timer.Reset(l.duration)
}
