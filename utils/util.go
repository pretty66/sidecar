package utils

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/openmsp/sidecar/pkg/resource"
	"github.com/openmsp/sidecar/pkg/resource/filesource"
	"github.com/openmsp/sidecar/utils/bodyBuffer"

	"github.com/openmsp/cilog"
	"github.com/shopspring/decimal"
)

func FloatToString(num float64) string {
	// to convert a float number to a string
	return strconv.FormatFloat(num, 'f', 2, 64)
}

func GetContainerMemory(rs resource.Resource) (int64, int64) {
	if rs == nil {
		rs = filesource.NewFileSource()
	}
	mStat, err := rs.CurrentMemStat()
	if err != nil {
		cilog.LogWarnf(cilog.LogNameSidecar, "heartBeatReport metrics.getContainerMemory error", err)
		return 0, 0
	}
	var total, rss string
	total = strconv.FormatUint(mStat.Total, 10)
	rss = strconv.FormatUint(mStat.RSS, 10)
	t, _ := decimal.NewFromString(total)
	r, _ := decimal.NewFromString(rss)
	d := decimal.NewFromInt32(1024 * 1024)
	return t.Div(d).IntPart(), r.Div(d).IntPart()
}

func IsFileExist(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func GoWithRecover(handler func(), recoverHandler func(r interface{})) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				cilog.LogErrorf(cilog.LogNameSidecar, "%s goroutine panic: %v\n%s\n", time.Now().Format("2006-01-02 15:04:05"), r, string(debug.Stack()))
				if recoverHandler != nil {
					go func() {
						defer func() {
							if p := recover(); p != nil {
								log.Printf("recover goroutine panic:%v\n%s\n", p, string(debug.Stack()))
							}
						}()
						recoverHandler(r)
					}()
				}
			}
		}()
		handler()
	}()
}

func TimeMs() int64 {
	return time.Now().UnixNano() / 1e6
}

func JSONEncode(param interface{}) []byte {
	b, err := json.Marshal(param)
	if err != nil {
		pc, f, l, _ := runtime.Caller(1)
		fc := runtime.FuncForPC(pc)
		cilog.LogWarnf(cilog.LogNameSidecar, "json_encode err: file:%s, line:%s, function name:%s, err:%s", f, l, fc.Name(), err.Error())
		return []byte{}
	}
	return b
}

func JSONToMap(jsonStr string) (map[string]string, error) {
	m := make(map[string]string)
	err := json.Unmarshal([]byte(jsonStr), &m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func IsConnectionRefused(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "connection refused")
}

func GetContainerCPU(rs resource.Resource, cpu chan float64, times time.Duration) {
	if times == 0 {
		times = time.Millisecond * 250 // 250ms
	}
	rs.GetCPUStat(times, func(stat *resource.CPUStat, err error) {
		if err != nil {
			cpu <- 0
		} else {
			cpu <- stat.Usage
		}
	})
}

func GetContainerDisk(rs resource.Resource, disk chan resource.DiskStat, times time.Duration) {
	if times == 0 {
		times = time.Second
	}
	rs.CurrentDiskStat(times, func(stat *resource.DiskStat, err error) {
		if err != nil {
			disk <- resource.DiskStat{}
		} else {
			disk <- *stat
		}
	})
}

func GetContainerNetwork(rs resource.Resource, net chan resource.NetworkStat, times time.Duration) {
	if times == 0 {
		times = time.Second
	}
	rs.CurrentNetworkStat(times, func(stat *resource.NetworkStat, err error) {
		if err != nil {
			net <- resource.NetworkStat{}
		} else {
			net <- *stat
		}
	})
}

func InArray(in string, array []string) bool {
	for k := range array {
		if in == array[k] {
			return true
		}
	}
	return false
}

func GetTraceIDNetHTTP(header http.Header) string {
	traceID := header.Get("sw8")
	if len(traceID) == 0 {
		return ""
	}
	sw8Array := strings.Split(traceID, "-")

	if len(sw8Array) >= 2 {
		if traceID, err := base64.StdEncoding.DecodeString(sw8Array[1]); err == nil {
			return string(traceID)
		}
	}
	return ""
}

func GetPort(host string) string {
	_, port, _ := net.SplitHostPort(host)
	return port
}

func GetResponseBody(w http.ResponseWriter) []byte {
	b, ok := w.(*bodyBuffer.BodyWriter)
	if !ok {
		return nil
	}
	return b.Body.Bytes()
}
