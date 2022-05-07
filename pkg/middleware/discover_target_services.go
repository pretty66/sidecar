package middleware

import (
	"errors"
	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/pkg/out"
	"github.com/openmsp/sidecar/sc"
	"github.com/openmsp/sidecar/utils"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/openmsp/cilog"

	"github.com/labstack/echo/v4"
	"github.com/openmsp/sesdk/discovery"
)

func NetHTTPDiscoverTargetServices(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		if ctx.Get(confer.HTTPRouterFilter) != nil {
			err := next(ctx)
			if ctx.Response().Status > http.StatusBadRequest && err == nil {
				err = errors.New(" status " + strconv.Itoa(ctx.Response().Status))
			}
			if err != nil {
				app := ctx.Get(confer.AppInfoKey).(*confer.RemoteApp)
				return out.HTTPJsonResponseError(ctx, app, err)
			}
			return nil
		}

		app := ctx.Get(confer.AppInfoKey).(*confer.RemoteApp)
		var err error
		for {
			app.Host, app.Hostname, app.Scheme, app.IsRetry, err = getAvailableEndpoint(app.AppID)
			if err != nil {
				if err == discovery.ErrInstanceNotFound && sc.Sc().Confer.Opts.Knative.EnableScaleZero {
					app.Host, app.Hostname, app.Scheme, app.IsKnative = "activator-service.knative-serving:80", "activator-service", "http", true
					app.ExtraHeader = map[string]string{"Knative-Serving-Namespace": ctx.Get(confer.AppNamespaceKey).(string), "Knative-Serving-Revision": app.AppID} // todo revision check
				} else {
					return out.HTTPJsonResponseError(ctx, app, err)
				}
			}
			ctx.Set(confer.AppInfoKey, app)
			err = next(ctx)
			if ctx.Response().Status > http.StatusBadRequest {
				err = errors.New(" status " + strconv.Itoa(ctx.Response().Status))
			}
			if err != nil {
				if utils.IsConnectionRefused(err) && app.IsRetry {
					sc.Cache().Set(app.Hostname, app.Host, time.Second*30)
					cilog.LogWarnf(cilog.LogNameSidecar, "connection refused retry:%s, %v", app, err)
					continue
				} else {
					return out.HTTPJsonResponseError(ctx, app, err)
				}
			}
			return nil
		}
	}
}

func getAvailableEndpoint(uniqueID string) (host, hostname, scheme string, isRetry bool, err error) {
	forNum := 1
	var ep *discovery.Endpoint
	for {
		ep, err = sc.Discovery().GetEndpoint(uniqueID)
		if err != nil {
			return
		}
		var h *url.URL
		h, err = url.Parse(ep.Host)
		if err != nil {
			return
		}
		host = h.Host + h.Path
		hostname = ep.Hostname
		scheme = h.Scheme

		if ep.Total == 1 {
			return
		}
		if _, ok := sc.Cache().Get(hostname); !ok {
			isRetry = forNum < ep.Total
			return
		}
		if ep.Total <= forNum {
			err = errors.New("target service is not available.")
			return
		}
		forNum++
	}
}
