package middleware

import (
	"bytes"
	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/utils/bodyBuffer"
	"io/ioutil"
	"sync"

	"github.com/labstack/echo/v4"
)

var (
	requestBodyPool  = sync.Pool{}
	responseBodyPool = sync.Pool{}
)

// Default response body replication maximum value
var maxCloneBodySize int64 = 4096

// InitBodyLimit Initialize the body configuration
// cloneBodySize The maximum body that can be cloned
func InitBodyLimit(cloneBodySize int) {
	if cloneBodySize > 0 {
		maxCloneBodySize = int64(cloneBodySize)
	}

	requestBodyPool.New = func() interface{} {
		return &bytes.Buffer{}
	}
	responseBodyPool.New = func() interface{} {
		return bodyBuffer.NewBodWriter(int(maxCloneBodySize))
	}
}

func EchoBodyLogMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		// request body
		reqLen := ctx.Request().ContentLength
		if reqLen > 0 && reqLen < maxCloneBodySize {
			reqBuf := requestBodyPool.Get().(*bytes.Buffer)
			_, _ = reqBuf.ReadFrom(ctx.Request().Body)
			ctx.Request().Body = ioutil.NopCloser(reqBuf)
			ctx.Set(confer.REQUEST_BODY_KEY, reqBuf.Bytes())

			defer func() {
				reqBuf.Reset()
				requestBodyPool.Put(reqBuf)
			}()
		}
		// response body
		buf := responseBodyPool.Get().(*bodyBuffer.BodyWriter)
		buf.ResponseWriter = ctx.Response().Writer
		ctx.Response().Writer = buf
		defer func() {
			buf.Reset()
			responseBodyPool.Put(buf)
		}()
		return next(ctx)
	}
}
