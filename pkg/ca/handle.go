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
	CA_CERT_STATUS_REDIS_FIELD_KEY = "ca"
	CA_CERT_FORBID                 = "forbid"
	CA_CERT_RECOVER                = "recover"
)

type CertStatus struct {
	status     atomic.Value
	configChan <-chan []byte
}

func (cs *CertStatus) HTTPCertStatusVerify(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !cs.status.Load().(bool) {
			return out.HTTPJsonOutputByte(c.Response(), http.StatusOK, errno.CaCertRevoke.Add("Ocsp authentication failed and the certificate was revoked. Procedure").Encode())
		}
		return next(c)
	}
}

func InitCaCertStatusVerify(sidecar *sc.SC) *CertStatus {
	cs := &CertStatus{}
	cs.status.Store(true)

	cs.configChan = sidecar.RegisterConfigWatcher(CA_CERT_STATUS_REDIS_FIELD_KEY, func(key, value []byte) bool {
		return string(key) == CA_CERT_STATUS_REDIS_FIELD_KEY
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
			if !util.InArray(string(v), []string{CA_CERT_FORBID, CA_CERT_RECOVER}) {
				continue
			}
			// true == recover
			if cs.status.Load().(bool) && string(v) == CA_CERT_RECOVER {
				continue
			}
			// false == forbid
			if !cs.status.Load().(bool) && string(v) == CA_CERT_FORBID {
				continue
			}

			cilog.LogWarnf(cilog.LogNameSidecar, "CA certificate status changed to：%s", string(v))
			cs.status.Store(string(v) == CA_CERT_RECOVER)

			switch string(v) {
			case CA_CERT_RECOVER:
				caclient.AllowOcspRequests()
			case CA_CERT_FORBID:
				caclient.BlockOcspRequests()
			}
		}
	}, func(r interface{}) {
		time.Sleep(time.Second)
		cs.listenStatusChange()
	})
}
