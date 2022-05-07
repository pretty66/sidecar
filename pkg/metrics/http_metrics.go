package metrics

import (
	"github.com/labstack/echo/v4"
	"github.com/rcrowley/go-metrics"
)

const (
	// total number of requests
	InflowRequestTotalMetrics = "request_total_inflow"
	InflowRequestErrMetrics   = "request_err_total_inflow"
	// infludb table name
)

func init() {
	// total number of requests
	_ = metrics.Register(InflowRequestTotalMetrics, metrics.NewCounter())
	// number of all error responses
	_ = metrics.Register(InflowRequestErrMetrics, metrics.NewCounter())
}

// timing data reporting middleware
func NetHTTPMetrics(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		metrics.Get(InflowRequestTotalMetrics).(metrics.Counter).Inc(1)
		err := next(ctx)
		if err != nil || ctx.Response().Status >= 400 {
			metrics.Get(InflowRequestErrMetrics).(metrics.Counter).Inc(1)
		}
		return err
	}
}
