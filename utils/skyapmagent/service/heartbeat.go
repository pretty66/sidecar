package service

import (
	"context"
	"syscall"
	"time"

	v3 "github.com/SkyAPM/go2sky/reporter/grpc/management"
	log "github.com/openmsp/cilog"
)

func (t *Agent) heartbeat() {
	t.registerCacheLock.Lock()
	// nolint
	var heartList []registerCache
	for pid, bind := range t.registerCache {
		if err := syscall.Kill(pid, 0); err != nil {
			delete(t.registerCache, pid)
			continue
		}
		heartList = append(heartList, bind)
	}
	t.registerCacheLock.Unlock()

	for _, bind := range heartList {
		if t.version == 8 {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
			defer cancel()

			_, err := t.grpcReport.GetManagementClient().KeepAlive(ctx, &v3.InstancePingPkg{
				Service:         bind.Service,
				ServiceInstance: bind.ServiceInstance,
			})
			if err != nil {
				log.LogErrorw(log.LogNameSidecar, "apm-agent heartbeat:", err)
			}
		}
	}
}
