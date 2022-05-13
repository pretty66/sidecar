package acl

import (
	"net/http"
	"sync"
	"time"

	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/pkg/errno"
	"github.com/openmsp/sidecar/pkg/out"
	"github.com/openmsp/sidecar/pkg/remoteconfer"
	"github.com/openmsp/sidecar/sc"
	"github.com/openmsp/sidecar/utils"

	"github.com/labstack/echo/v4"

	"github.com/openmsp/cilog"
)

type FlowVerification struct {
	rule       *Rules
	rlock      sync.RWMutex
	configChan <-chan []byte
}

var DefaultACL *FlowVerification

func InitACL() *FlowVerification {
	if DefaultACL != nil {
		return DefaultACL
	}
	fv := &FlowVerification{}
	fv.configChan = sc.Sc().RegisterConfigWatcher(remoteconfer.MspServiceRuleACL, func(key, value []byte) bool {
		return string(key) == remoteconfer.MspServiceRuleACL
	}, 1)
	utils.GoWithRecover(func() {
		fv.listenChanges()
	}, func(r interface{}) {
		time.Sleep(time.Second * 3)
		fv.listenChanges()
	})
	DefaultACL = fv
	return DefaultACL
}

func NetHTTPRuleMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return DefaultACL.NetHTTPRuleMiddleware(next)
}

func (fv *FlowVerification) NetHTTPRuleMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		fromUniqueID := ctx.Request().Header.Get(confer.REQUEST_HEADER_FROM_UNIQUEID)
		err := fv.Verify(fromUniqueID, ctx.Request().Method, ctx.Request().URL.Path, func(key string) (string, bool) {
			h := ctx.Request().Header.Get(key)
			return h, h != ""
		})
		if err != nil {
			return out.HTTPJsonRequestError(ctx.Response(), http.StatusForbidden, err)
		}
		return next(ctx)
	}
}

func (fv *FlowVerification) Verify(fromUniqueID, method, path string, head HeaderFunc) error {
	fv.rlock.RLock()
	defer fv.rlock.RUnlock()
	if fv.rule == nil {
		return nil
	}
	if len(fromUniqueID) == 0 {
		return errno.RequestForbidden
	}
	return fv.rule.Verify(fromUniqueID, method, path, head)
}

func (fv *FlowVerification) listenChanges() {
	for conf := range fv.configChan {
		if len(conf) == 0 {
			fv.clear()
			continue
		}
		arule, err := NewACLRuleByConfig(conf)
		if err != nil {
			cilog.LogWarnw(cilog.LogNameSidecar, err.Error())
			continue
		}
		fv.reload(arule)
		cilog.LogInfof(cilog.LogNameSidecar, "induction acl configuration：%s", string(conf))
	}
}

func (fv *FlowVerification) reload(r *Rules) {
	fv.rlock.Lock()
	fv.rule = r
	fv.rlock.Unlock()
}

func (fv *FlowVerification) clear() {
	var is bool
	fv.rlock.Lock()
	is = fv.rule == nil
	fv.rule = nil
	fv.rlock.Unlock()
	if !is {
		cilog.LogInfow(cilog.LogNameSidecar, "clear all acl configurations！")
	}
}
