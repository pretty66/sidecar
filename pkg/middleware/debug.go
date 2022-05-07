package middleware

import (
	"bytes"
	"log"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/openmsp/cilog"
)

func NetHTTPDebugMiddleware(prefix string) func(echo.HandlerFunc) echo.HandlerFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			t1 := time.Now()
			uri := bytes.NewBuffer(nil)
			if c.Request().TLS == nil {
				uri.WriteString("http://")
			} else {
				uri.WriteString("https://")
			}
			uri.WriteString(c.Request().Host)
			uri.WriteString(c.Request().RequestURI)
			err := next(c)
			if err != nil {
				log.Println(err)
			}
			t := time.Now().Sub(t1)
			if t.Seconds() > 20 {
				cilog.LogWarnf(cilog.LogNameSidecar, "%s request time %s, url: %s, response status code：%d, request body length：%d, response body size：%d",
					prefix,
					t.String(),
					uri.String(),
					c.Response().Status,
					c.Request().ContentLength,
					c.Response().Size,
				)
			}
			return err
		}
	}
}
