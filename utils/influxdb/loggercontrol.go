package influxdb

import "time"

type loggerControl struct {
	duration time.Duration
	timer    *time.Timer
}

func NewLoggerControl(duration time.Duration) *loggerControl {
	return &loggerControl{
		duration: duration,
		timer:    time.NewTimer(duration),
	}
}

func (l *loggerControl) Reset() {
	l.timer.Reset(l.duration)
}
