package middleware

import (
	"errors"
	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/pkg/out"
	"strings"

	"github.com/labstack/echo/v4"
)

func ParseTargetAppInfo(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		id := findTargetID(ctx)
		if id == "" {
			return out.HTTPJsonResponseError(ctx, nil, errors.New("app id not found"))
		}
		var (
			appID     string
			namespace string
		)
		items := strings.Split(id, ".")
		if len(items) == 1 {
			appID, namespace = items[0], getNameSpace()
		} else if len(items) == 2 {
			appID, namespace = items[0], items[1]
		} else {
			return out.HTTPJsonResponseError(ctx, nil, errors.New("app id parse error"))
		}

		ctx.Set(confer.AppIDKey, appID)
		ctx.Set(confer.AppNamespaceKey, namespace)
		ctx.Set(confer.AppInfoKey, &confer.RemoteApp{AppID: appID, Path: ctx.Request().URL.Path})
		return next(ctx)
	}
}

// findTargetID tries to find ID of the target service from the following three places:
// 1. HTTP header: 'app-id'.
func findTargetID(ctx echo.Context) string {
	return ctx.Request().Header.Get(confer.HeaderAppID)
}

func getNameSpace() string {
	return confer.Global().Opts.MiscroServiceInfo.Namespace
}
