package router

import (
	"errors"
	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/pkg/out"
	"net/url"

	"github.com/labstack/echo/v4"
)

func (r *Rule) NetHTTPRuleMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		appid := ctx.Get(confer.AppIDKey).(string)
		remoteURL, err := r.Rule(appid, ctx.Request().Method, ctx.Request().URL.Path, func(key string) (string, bool) {
			h := ctx.Request().Header.Get(key)
			return h, h != ""
		}, func(shadow *ShadowRequest) {
			shadow.NetHTTPShadow(r.shadowRequestQueue, ctx.Request().URL.Path, ctx.Request())
		})
		if err != nil {
			return out.HTTPJsonResponseError(ctx, nil, err)
		}
		if remoteURL != "" {
			u, err := url.Parse(remoteURL)
			if err != nil {
				return out.HTTPJsonResponseError(ctx, nil, errors.New("The destination service address matched by traffic traction is incorrect. Procedure："+err.Error()))
			}
			ctx.Set(confer.AppInfoKey, &confer.RemoteApp{AppID: appid, Scheme: u.Scheme, Host: u.Host + u.Path})

			ctx.Set(confer.HTTPRouterFilter, true)
		}

		return next(ctx)
	}
}

func (r *Rule) Rule(uniqueID, method, path string, headerFunc HeaderFunc, shadowFunc func(request *ShadowRequest)) (remoteURL string, err error) {
	r.lock.RLock()
	rule, ok := r.ruleList[uniqueID]
	r.lock.RUnlock()
	if !ok {
		return
	}
	for k := range rule.entity {
		if !rule.entity[k].Match.Matches(path, method, headerFunc) {
			continue
		}
		if rule.entity[k].Mirror != nil {
			shadowFunc(rule.entity[k].Mirror)
		}
		if rule.entity[k].Distribution != nil {
			remoteURL, err = rule.entity[k].Distribution.Weight()
			break
		}
	}
	return
}
