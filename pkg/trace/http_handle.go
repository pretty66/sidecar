package trace

import (
	"context"
	"github.com/openmsp/sidecar/pkg/confer"
	util "github.com/openmsp/sidecar/utils"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/SkyAPM/go2sky"
	"github.com/SkyAPM/go2sky/propagation"
	v3 "github.com/SkyAPM/go2sky/reporter/grpc/language-agent"
	"github.com/labstack/echo/v4"
	"github.com/openmsp/cilog"
)

func (t *SkyTrace) HTTPInflowTraceHandle(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if t.close.Load().(bool) {
			return next(c)
		}
		sw8 := c.Request().Header.Get(propagation.Header)
		uri := c.Request().URL.Path
		if uri == "/healthcheck" {
			return next(c)
		}

		tracer := t.Tracer
		if tracer == nil {
			return nil
		}
		span, entryCtx, err := tracer.CreateEntrySpan(context.TODO(), uri, func() (string, error) {
			return sw8, nil
		})
		if err != nil {
			cilog.LogWarnf(cilog.LogNameSidecar, "inflow trace CreateEntrySpan error sw8:"+sw8, err)
			return next(c)
		}
		if sw8 == "" {
			s := span.(go2sky.ReportedSpan)
			sw8 = (&propagation.SpanContext{
				TraceID:               s.Context().TraceID,
				ParentSegmentID:       s.Context().ParentSegmentID,
				ParentService:         confer.Global().Opts.MiscroServiceInfo.UniqueID,
				ParentServiceInstance: confer.Global().Opts.MiscroServiceInfo.UniqueID,
				ParentEndpoint:        uri,
				AddressUsedAtClient:   confer.Global().Opts.MiscroServiceInfo.EndPointAddress,
				ParentSpanID:          -1,
				Sample:                1,
			}).EncodeSW8()
		}

		skys := &skySpan{entryCtx: entryCtx}
		t.lock.Lock()
		t.m[sw8] = skys
		t.lock.Unlock()
		c.Request().Header.Set(propagation.Header, sw8)
		err = next(c)

		span.SetSpanLayer(v3.SpanLayer_Http) // golang
		span.SetPeer(t.inEndpoint)
		span.SetOperationName(uri)
		span.SetComponent(componentIDGOHttpServer)
		span.Tag(go2sky.TagHTTPMethod, c.Request().Method)
		span.Tag(go2sky.TagURL, uri)
		span.Tag("go_version", runtime.Version())
		c.Set(spanContextKey, span)
		status := c.Response().Status
		span.Tag(go2sky.TagStatusCode, strconv.Itoa(status))
		if err != nil {
			span.Error(time.Now(), "error", err.Error())
		} else if status >= http.StatusBadRequest {
			// response body
			if c.Response().Size <= 1024 {
				span.Error(time.Now(), "response", string(util.GetResponseBody(c.Response().Writer)))
			} else {
				span.Error(time.Now(), "response", string(util.GetResponseBody(c.Response().Writer)[:1024]))
			}
		}
		span.End()
		t.lock.Lock()
		delete(t.m, sw8)
		t.lock.Unlock()
		return err
	}
}

func (t *SkyTrace) NetHTTPOutflowTraceHandle(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if t.close.Load().(bool) {
			return next(c)
		}
		uri := c.Request().URL.Path
		if uri == "/healthcheck" {
			return next(c)
		}
		sw8 := c.Request().Header.Get(propagation.Header)
		if sw8 == "" {
			return next(c)
		}

		t.lock.RLock()
		entrySpan, ok := t.m[sw8]
		t.lock.RUnlock()
		if !ok {
			return next(c)
		}

		tracer := t.Tracer
		if tracer == nil {
			return nil
		}
		span, err := tracer.CreateExitSpan(entrySpan.entryCtx, uri, confer.Global().Opts.MiscroServiceInfo.EndPointAddress, func(header string) error {
			c.Request().Header.Set(propagation.Header, header)
			return nil
		})
		if err != nil {
			cilog.LogErrorw(cilog.LogNameSidecar, "Insensitive traffic management: Failed to create egress link data！", err)
			return next(c)
		}

		span.SetSpanLayer(v3.SpanLayer_Http)
		span.SetOperationName(uri)
		span.SetComponent(componentIDGOHttpClient)
		span.Tag(go2sky.TagHTTPMethod, c.Request().Method)
		span.Tag(go2sky.TagURL, uri)
		setIdgTag2Span(span, entrySpan)
		c.Set(spanContextKey, span)
		err = next(c)
		status := c.Response().Status
		span.Tag(go2sky.TagStatusCode, strconv.Itoa(status))
		if err != nil {
			span.Error(time.Now(), "error", err.Error())
		} else if status >= http.StatusBadRequest {
			// response body
			if c.Response().Size <= 1024 {
				span.Error(time.Now(), "response", string(util.GetResponseBody(c.Response().Writer)))
			} else {
				span.Error(time.Now(), "response", string(util.GetResponseBody(c.Response().Writer)[:1024]))
			}
		}
		span.End()
		return err
	}
}

func (t *SkyTrace) HTTPEventInflowTraceHandle(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sw8 := c.Request().Header.Get(propagation.Header)
		if sw8 != "" {
			return t.NetHTTPOutflowTraceHandle(next)(c)
		}
		return t.HTTPInflowTraceHandle(next)(c)
	}
}
