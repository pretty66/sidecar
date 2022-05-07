package service

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"runtime"
	"strconv"
	"time"

	nc3 "github.com/SkyAPM/go2sky/reporter/grpc/common"
	nm3 "github.com/SkyAPM/go2sky/reporter/grpc/management"
	"github.com/google/uuid"
	log "github.com/openmsp/cilog"
)

type registerCache struct {
	Version         int
	AppID           int32
	InstanceID      int32
	UUID            string
	Service         string
	ServiceInstance string
}

type registerReq struct {
	AppCode string `json:"app_code"`
	Pid     int    `json:"pid"`
	Version int    `json:"version"`
}

func ip4s() []string {
	ipv4s, addErr := net.InterfaceAddrs()
	var ips []string
	if addErr == nil {
		for _, i := range ipv4s {
			if ipnet, ok := i.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.IP.String())
				}
			}
		}
	}
	return ips
}

func (t *Agent) recoverRegister(info trace) {
	t.registerCacheLock.RLock()
	_, ok := t.registerCache[info.Pid]
	t.registerCacheLock.RUnlock()

	if !ok {
		t.registerCacheLock.Lock()
		t.registerCache[info.Pid] = registerCache{
			Version:    info.Version,
			AppID:      info.ApplicationID,
			InstanceID: info.ApplicationInstance,
			UUID:       info.UUID,
		}
		t.registerCacheLock.Unlock()
	}
}

func (t *Agent) doRegister(r *register) {
	info := registerReq{}
	err := json.Unmarshal([]byte(r.body), &info)
	if err != nil {
		log.LogErrorw(log.LogNameSidecar, "register json decode error", err)
		r.c.Write([]byte(""))
		return
	}

	pid := info.Pid
	t.registerCacheLock.RLock()
	bind, ok := t.registerCache[pid]
	t.registerCacheLock.RUnlock()
	if ok {
		if t.version == 8 {
			r.c.Write([]byte(info.AppCode + "," + bind.UUID + "," + bind.UUID))
		}
		return
	}

	t.registerCacheLock.Lock()
	defer t.registerCacheLock.Unlock()

	if _, ok := t.registerCache[pid]; !ok {
		log.LogInfof(log.LogNameSidecar, "start register pid %d used SkyWalking v%d", pid, info.Version)
		regAppStatus := false
		var appID int32
		var appInsID int32
		agentUUID := uuid.New().String()

		if t.version == 8 {
			c := t.grpcReport.GetManagementClient()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
			defer cancel()

			req := &nm3.InstanceProperties{
				Service:         info.AppCode,
				ServiceInstance: uuid.New().String(),
			}

			req.Properties = append(req.Properties, &nc3.KeyStringValuePair{
				Key:   "os_name",
				Value: runtime.GOOS,
			})

			hostName, _ := os.Hostname()
			req.Properties = append(req.Properties, &nc3.KeyStringValuePair{
				Key:   "host_name",
				Value: hostName,
			}, &nc3.KeyStringValuePair{
				Key:   "process_no",
				Value: strconv.Itoa(pid),
			}, &nc3.KeyStringValuePair{
				Key:   "language",
				Value: "php",
			})

			for _, ip := range ip4s() {
				req.Properties = append(req.Properties, &nc3.KeyStringValuePair{
					Key:   "ipv4",
					Value: ip,
				})
			}

			_, err = c.ReportInstanceProperties(ctx, req)

			if err != nil {
				log.LogErrorw(log.LogNameSidecar, "report instance properties:", err)
			} else {
				t.registerCache[pid] = registerCache{
					Version:         t.version,
					AppID:           1,
					InstanceID:      1,
					UUID:            req.ServiceInstance,
					Service:         info.AppCode,
					ServiceInstance: req.ServiceInstance,
				}
				log.LogInfof(log.LogNameSidecar, "registered pid %d, service %s service instance %s", pid, info.AppCode, req.ServiceInstance)
			}
			return
		}

		if regAppStatus {
			if appInsID != 0 {
				t.registerCache[pid] = registerCache{
					Version:    info.Version,
					AppID:      appID,
					InstanceID: appInsID,
					UUID:       agentUUID,
				}
				log.LogInfof(log.LogNameSidecar, "register pid %d appid %d insId %d", pid, appID, appInsID)
			}
		} else {
			log.LogErrorw(log.LogNameSidecar, "register error:", err)
		}
	}
}
