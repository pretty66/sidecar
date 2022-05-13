package ca

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/openmsp/sidecar/pkg/errno"
	"github.com/openmsp/sidecar/pkg/out"
	"github.com/openmsp/sidecar/sc"
	util "github.com/openmsp/sidecar/utils"

	"github.com/IceFireDB/IceFireDB-Proxy/utils"
	"github.com/labstack/echo/v4"
	"github.com/openmsp/cilog"
	"github.com/ztalab/ZACA/pkg/caclient"
)

const (
	CaCertStatusRedisFieldKey = "ca"
	CaCertForbid              = "forbid"
	CaCertRecover             = "recover"
)

type CertStatus struct {
	status     atomic.Value
	configChan <-chan []byte
}

func (cs *CertStatus) HTTPCertStatusVerify(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !cs.status.Load().(bool) {
			return out.HTTPJsonOutputByte(c.Response(), http.StatusOK, errno.ErrCaCertRevoke.Add("Ocsp authentication failed and the certificate was revoked. Procedure").Encode())
		}
		return next(c)
	}
}

func InitCaCertStatusVerify(sidecar *sc.SC) *CertStatus {
	cs := &CertStatus{}
	cs.status.Store(true)

	cs.configChan = sidecar.RegisterConfigWatcher(CaCertStatusRedisFieldKey, func(key, value []byte) bool {
		return string(key) == CaCertStatusRedisFieldKey
	}, 1)
	cs.listenStatusChange()
	return cs
}

func (cs *CertStatus) listenStatusChange() {
	utils.GoWithRecover(func() {
		for v := range cs.configChan {
			if len(v) == 0 {
				continue
			}
			if !util.InArray(string(v), []string{CaCertForbid, CaCertRecover}) {
				continue
			}
			// true == recover
			if cs.status.Load().(bool) && string(v) == CaCertRecover {
				continue
			}
			// false == forbid
			if !cs.status.Load().(bool) && string(v) == CaCertForbid {
				continue
			}

			cilog.LogWarnf(cilog.LogNameSidecar, "CA certificate status changed to：%s", string(v))
			cs.status.Store(string(v) == CaCertRecover)

			switch string(v) {
			case CaCertRecover:
				caclient.AllowOcspRequests()
			case CaCertForbid:
				caclient.BlockOcspRequests()
			}
		}
	}, func(r interface{}) {
		time.Sleep(time.Second)
		cs.listenStatusChange()
	})
}
