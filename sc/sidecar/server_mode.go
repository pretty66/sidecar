package sidecar

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/utils"

	"github.com/openmsp/cilog"
)

func (hp *httpReverseProxyServer) restart() {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*5)
	defer cancel()
	_ = hp.inflow.Server.Shutdown(ctx)
	_ = hp.inflow.Server.Close()
	go hp.startInflow()
}

func (hp *httpReverseProxyServer) httpProxyListenForConfig(configChan <-chan []byte) {
	utils.GoWithRecover(func() {
		for newMode := range configChan {
			if len(newMode) == 0 || !utils.InArray(string(newMode), []string{confer.HTTP, confer.HTTPS, confer.MTLS}) {
				continue
			}
			oldMode := hp.InflowProtocol.Load().(string)
			newMode := string(newMode)
			if newMode == oldMode {
				continue
			}
			scheme := confer.HTTP
			if utils.InArray(newMode, []string{confer.HTTPS, confer.MTLS}) {
				scheme = confer.HTTPS
			}
			port := utils.GetPort(hp.Confer.Opts.TrafficInflow.BindAddress)

			hp.InflowProtocol.Store(newMode)
			hp.Discovery.Cancel(hp.Instance.AppID)
			hp.restart()
			time.Sleep(time.Second)
			hp.Instance.Metadata["mode"] = newMode
			hp.Instance.Addrs = []string{fmt.Sprintf("%s://%s:%s", scheme, hp.Confer.Opts.MiscroServiceInfo.PodIP, port)}
			_, err := hp.Discovery.Register(context.TODO(), hp.Instance)
			if err != nil {
				cilog.LogErrorw(cilog.LogNameSidecar, "error updating the traffic access mode", err)
				continue
			}
			cilog.LogWarnf(cilog.LogNameSidecar, "update traffic access mode by %s => %s", oldMode, newMode)
			log.Printf("update traffic access mode by %s => %s \n", oldMode, newMode)
		}
	}, func(r interface{}) {
		time.Sleep(3 * time.Second)
		hp.httpProxyListenForConfig(configChan)
	})
}
