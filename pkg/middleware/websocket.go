package middleware

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/openmsp/cilog"
)

func NetHTTPWebsocketProxy(globalCtx context.Context, targetAddr string) func(echo.HandlerFunc) echo.HandlerFunc {
	origin := fmt.Sprintf("http://%s", targetAddr)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			if !ctx.IsWebSocket() {
				return next(ctx)
			}
			req := ctx.Request().Clone(ctx.Request().Context())
			// req.URL.Path, req.URL.RawPath, req.RequestURI = wp.defaultPath, wp.defaultPath, wp.defaultPath
			req.Host = targetAddr
			req.Header.Set("Origin", origin)
			remoteConn, err := net.Dial("tcp", targetAddr)
			if err != nil {
				cilog.LogWarnf(cilog.LogNameSidecar, "websocket proxy net.Dial err:%v", err)
				http.Error(ctx.Response(), fmt.Sprintf("websocket proxy net.Dial err:%v", err), http.StatusInternalServerError)
				return nil
			}
			defer remoteConn.Close()
			err = req.Write(remoteConn)
			if err != nil {
				http.Error(ctx.Response(), fmt.Sprintf("websocket proxy remote write err:%v", err), http.StatusInternalServerError)
				return nil
			}
			hijacker, ok := ctx.Response().Writer.(http.Hijacker)
			if !ok {
				cilog.LogWarnw(cilog.LogNameSidecar, "websocket hijacking request failed")
				http.Error(ctx.Response(), "websocket proxy Hijacker fail", http.StatusInternalServerError)
				return nil
			}
			conn, _, err := hijacker.Hijack()
			if err != nil {
				cilog.LogWarnf(cilog.LogNameSidecar, "websocket hijacking request failed:%v", err)
				http.Error(ctx.Response(), fmt.Sprintf("websocket proxy Hijacker err:%v", err), http.StatusInternalServerError)
				return nil
			}
			defer conn.Close()
			errChan := make(chan error, 2)
			copyConn := func(a, b net.Conn) {
				_, err := io.Copy(a, b)
				errChan <- err
			}
			go copyConn(conn, remoteConn) // response
			go copyConn(remoteConn, conn) // request
			select {
			case err = <-errChan:
			case <-globalCtx.Done():
			}
			return nil
		}
	}
}
