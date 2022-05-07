package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	network "knative.dev/networking/pkg"
	"knative.dev/serving/pkg/activator"
)

func KnativeMiddleWare(stats *network.RequestStats) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			r := ctx.Request()
			if network.IsKubeletProbe(r) {
				return next(ctx)
			}
			// Metrics for autoscaling.
			in, out := network.ReqIn, network.ReqOut
			if activator.Name == network.KnativeProxyHeader(r) {
				in, out = network.ProxiedIn, network.ProxiedOut
			}
			stats.HandleEvent(network.ReqEvent{Time: time.Now(), Type: in})
			defer func() {
				stats.HandleEvent(network.ReqEvent{Time: time.Now(), Type: out})
			}()
			network.RewriteHostOut(r)

			return next(ctx)
		}
	}
}
