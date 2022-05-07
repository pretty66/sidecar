package httpsource

import (
	"github.com/openmsp/cilog"
)

var logCount int64

func logMsg(msg string, err error) {
	if logCount > 10 {
		cilog.LogErrorw(cilog.LogNameSidecar, msg, err)
	} else {
		cilog.LogWarnf(cilog.LogNameSidecar, "%s, %s", msg, err.Error())
		logCount++
	}
}
