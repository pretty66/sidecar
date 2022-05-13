package middleware

import (
	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/pkg/errno"
	"github.com/openmsp/sidecar/pkg/out"
	util "github.com/openmsp/sidecar/utils"

	"github.com/labstack/echo/v4"
	"github.com/tidwall/gjson"
)

const (
	// parse validates the body maximum limit
	ResponseLimitBody = 65536
	// response body size alarm threshold
	ResponseLimitBodyWaring = 1024 * 1024
)

func NetHTTPResponseHandle(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		err := next(ctx)
		if err != nil || ctx.Response().Status > 400 {
			return err
		}
		if ctx.Response().Size > 0 && ctx.Response().Size <= ResponseLimitBody {
			res := util.GetResponseBody(ctx.Response().Writer)
			if len(res) > 0 && gjson.ValidBytes(res) {
				state := gjson.GetBytes(res, "state").String()
				if state != "1" {
					app := ctx.Get(confer.AppInfoKey).(*confer.RemoteApp)
					out.HTTPResponseWaring(ctx, app, errno.ErrTargetResponseWaring)
				}
			}
		}
		if ctx.Response().Size > ResponseLimitBodyWaring {
			app := ctx.Get(confer.AppInfoKey).(*confer.RemoteApp)
			out.HTTPResponseWaringBodySize(ctx, app, ctx.Response().Size)
		}
		return nil
	}
}
