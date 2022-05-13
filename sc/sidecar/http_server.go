package sidecar

import (
	"context"
	"errors"
	"github.com/openmsp/sidecar/pkg/metrics"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/openmsp/sidecar/pkg/acl"
	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/pkg/errno"
	localmidd "github.com/openmsp/sidecar/pkg/middleware"
	"github.com/openmsp/sidecar/pkg/out"
	"github.com/openmsp/sidecar/sc"
	"github.com/openmsp/sidecar/utils"
	"github.com/openmsp/sidecar/utils/networker"

	"github.com/openmsp/cilog"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// netpoll proxy mode
type httpReverseProxyServer struct {
	*TrafficProxy
	inflow        *networker.HTTPReverseProxy
	outflow       *networker.HTTPReverseProxy
	bufferPool    *networker.BufferPool
	inProxyConfig *networker.ReverseProxyServerOption
}

func startHTTPReverseProxy(tp *TrafficProxy) error {
	hproxy := &httpReverseProxyServer{
		TrafficProxy: tp,
		bufferPool:   networker.NewBufferPool(),
	}
	localmidd.InitBodyLimit(4096)

	go hproxy.startOutflow()
	go hproxy.startInflow()

	if tp.Confer.Opts.CaConfig.Enable {
		c := tp.SC.RegisterConfigWatcher("mode", func(key, value []byte) bool {
			return string(key) == "mode"
		}, 1)
		hproxy.httpProxyListenForConfig(c)
	}
	return nil
}

func (hp *httpReverseProxyServer) startInflow() {
	var err error
	if hp.inProxyConfig == nil {
		hp.inProxyConfig = &networker.ReverseProxyServerOption{
			ProtocolType:              strings.ToLower(hp.Confer.Opts.TrafficInflow.BindProtocolType),
			BindNetWork:               strings.ToLower(hp.Confer.Opts.TrafficInflow.BindNetWork),
			BindAddress:               hp.Confer.Opts.TrafficInflow.BindAddress,
			TargetAddress:             hp.Confer.Opts.TrafficInflow.TargetAddress,
			TargetProtocolType:        hp.Confer.Opts.TrafficInflow.TargetProtocolType,
			IdleTimeout:               time.Second * 60,
			TargetDialTimeout:         time.Duration(hp.Confer.Opts.TrafficInflow.TargetDialTimeout) * time.Second,
			TargetKeepAlive:           time.Duration(hp.Confer.Opts.TrafficInflow.TargetKeepAlive) * time.Second,
			TargetIdleConnTimeout:     time.Duration(hp.Confer.Opts.TrafficInflow.TargetIdleConnTimeout) * time.Second,
			TargetMaxIdleConnsPerHost: hp.Confer.Opts.TrafficInflow.TargetMaxIdleConnsPerHost,
		}
	}
	hp.inflow, err = networker.NewHTTPReverseProxyServer(hp.inProxyConfig)
	if err != nil {
		panic("inflow proxy init fail：" + err.Error())
	}
	mode := hp.InflowProtocol.Load().(string)
	if utils.InArray(mode, []string{confer.HTTPS, confer.MTLS}) {
		tlsc, e := hp.NewSidecarTLSServer(hp.InflowProtocol.Load().(string))
		if e != nil {
			panic("inflow ca init fail：" + e.Error())
		}
		hp.inflow.Server.TLSConfig = tlsc
	}
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	log.Printf("start inflow proxy: %s://%s => http://%s", mode, hp.inProxyConfig.BindAddress, hp.inProxyConfig.TargetAddress)
	ctx, cancel := context.WithCancel(hp.OfflineCtx)
	utils.GoWithRecover(func() {
		select {
		case <-ctx.Done():
			ctxs, canc := context.WithTimeout(context.Background(), time.Second*5)
			defer canc()
			err = hp.inflow.Server.Shutdown(ctxs)
			if err != nil {
				cilog.LogWarnf(cilog.LogNameSidecar, "inflow offline err：%v", err)
			}
		}
	}, nil)
	hp.inflow.Server.RegisterOnShutdown(func() {
		cancel()
	})
	hp.registerInflowRouter(e, mode)
	err = e.StartServer(hp.inflow.Server)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func (hp *httpReverseProxyServer) startOutflow() {
	var err error
	option := networker.ReverseProxyServerOption{
		ProtocolType:              strings.ToLower(hp.Confer.Opts.TrafficOutflow.BindProtocolType),
		BindNetWork:               strings.ToLower(hp.Confer.Opts.TrafficOutflow.BindNetWork),
		BindAddress:               hp.Confer.Opts.TrafficOutflow.BindAddress,
		IdleTimeout:               time.Second * 60,
		TargetDialTimeout:         time.Duration(hp.Confer.Opts.TrafficOutflow.TargetDialTimeout) * time.Second,
		TargetKeepAlive:           time.Duration(hp.Confer.Opts.TrafficOutflow.TargetKeepAlive) * time.Second,
		TargetIdleConnTimeout:     time.Duration(hp.Confer.Opts.TrafficOutflow.TargetIdleConnTimeout) * time.Second,
		TargetMaxIdleConnsPerHost: hp.Confer.Opts.TrafficOutflow.TargetMaxIdleConnsPerHost,
	}
	hp.outflow, err = networker.NewHTTPReverseProxyServer(&option)
	if err != nil {
		panic("outflow init fail：" + err.Error())
	}
	log.Println("start outflow proxy: http://" + option.BindAddress)

	if hp.Confer.Opts.CaConfig.Enable {
		transport := hp.outflow.Proxy.Transport.(*http.Transport)
		transport.TLSClientConfig, err = hp.NewSidecarMTLSClient("")
		if err != nil {
			panic("outflow ca init fail：" + err.Error())
		}
	}
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	hp.ShutdownWg.Add(1)
	go sc.ShutdownEchoServer(hp.Ctx, hp.outflow.Server, func(err error) {
		if err != nil {
			cilog.LogWarnf(cilog.LogNameSidecar, "outflow offline err：%v", err)
		}
		hp.ShutdownWg.Done()
	})

	e.Use(middleware.Recover())
	if hp.Metrics != nil {
		e.Any("/sidecar/metrics", hp.Metrics.MetricsHandlerFuncHTTP)
	}

	midd := []echo.MiddlewareFunc{
		hp.CaVerify.HTTPCertStatusVerify,
		localmidd.EchoBodyLogMiddleware,
	}
	if hp.Confer.Opts.Trace.Enable {
		midd = append(midd, hp.Trace.GetSky().NetHTTPOutflowTraceHandle)
	}

	midd = append(midd, localmidd.ParseTargetAppInfo,
		hp.Router.NetHTTPRuleMiddleware,
		localmidd.NetHTTPDiscoverTargetServices,
		hp.TrafficOutFuse.ExecuteCircuitBreakerNetHTTPMiddle,
		localmidd.NetHTTPResponseHandle,
		localmidd.NetHTTPDebugMiddleware("outflow"),
	)

	e.Any("*", hp.outflowProxy, midd...)

	err = e.StartServer(hp.outflow.Server)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic("start outflow proxy err：" + err.Error())
	}
}

func (hp *httpReverseProxyServer) registerInflowRouter(e *echo.Echo, protocol string) {
	e.Use(middleware.Recover())
	e.Use(acl.NetHTTPRuleMiddleware)
	g := e.Group("/sidecar")
	{
		g.GET("/cache", func(context echo.Context) error {
			body := map[string]interface{}{
				"state": 1,
				"msg":   "success",
				"data":  hp.Cache.Items(),
			}
			return out.HTTPJsonOutput(context.Response(), http.StatusOK, body)
		})
		g.GET("/clearScCache", func(c echo.Context) error {
			hp.Cache.Flush()
			return out.HTTPJsonOutputByte(c.Response(), http.StatusOK, nil)
		})
	}
	// websocket
	midd := []echo.MiddlewareFunc{}
	if hp.Confer.Opts.TrafficInflow.EnableWebsocket {
		midd = append(midd, localmidd.NetHTTPWebsocketProxy(hp.Ctx, hp.Confer.Opts.TrafficInflow.TargetAddress))
	}
	// knative traffic count
	if hp.Confer.Opts.Knative.Enable {
		e.Use(localmidd.KnativeMiddleWare(hp.State))
	}

	if hp.Confer.Opts.BBR.Enable {
		midd = append(midd, hp.BBRLimiter.HTTPBBRAllow)
	}
	if protocol != confer.HTTP {
		if hp.Confer.Opts.CaConfig.Enable && hp.CaVerify != nil {
			midd = append(midd, hp.CaVerify.HTTPCertStatusVerify)
		}
	}

	midd = append(midd,
		hp.TrafficInRateLimiter.HTTPRateLimiter,
		localmidd.EchoBodyLogMiddleware,
		hp.Trace.GetSky().HTTPInflowTraceHandle,
		metrics.NetHTTPMetrics,
		localmidd.NetHTTPDebugMiddleware("inflow"),
	)
	e.Any("*", hp.inflowProxy, midd...)
}

func (hp *httpReverseProxyServer) inflowProxy(ctx echo.Context) error {
	req := ctx.Request()
	for _, key := range confer.HopHeaders {
		req.Header.Del(key)
	}
	req.URL.Scheme = hp.inflow.Option.TargetProtocolType
	req.Host, req.URL.Host = hp.inflow.Option.TargetAddress, hp.inflow.Option.TargetAddress
	resp, err := hp.inflow.Proxy.Transport.RoundTrip(req)
	if err != nil {
		if ignoreErr(err) {
			return nil
		}
		return out.HTTPJsonResponseError(ctx, nil, errno.ErrRequest.Add(err.Error()))
	}
	defer resp.Body.Close()

	if fn := ctx.Get(confer.RespCacheProxyRespFunc); fn != nil {
		_ = fn.(confer.AfterProxyResponse)(resp)
	}
	for _, h := range confer.HopHeaders {
		resp.Header.Del(h)
	}

	for k, vv := range resp.Header {
		for _, v := range vv {
			ctx.Response().Header().Add(k, v)
		}
	}
	ctx.Response().WriteHeader(resp.StatusCode)

	buf := hp.bufferPool.Get()
	_, err = io.CopyBuffer(ctx.Response(), resp.Body, buf)
	hp.bufferPool.Put(buf)
	if err != nil {
		return out.InflowHTTPJsonRequestError(ctx.Response(), ctx.Request(), http.StatusOK, err)
	}
	return nil
}

func (hp *httpReverseProxyServer) outflowProxy(ctx echo.Context) error {
	app := ctx.Get(confer.AppInfoKey).(*confer.RemoteApp)

	req := ctx.Request()
	for _, h := range confer.HopHeaders {
		req.Header.Del(h)
	}

	req.URL.Scheme = app.Scheme
	req.Host, req.URL.Host = app.Host, app.Host
	if app.Path != "" {
		req.RequestURI, req.URL.Path, req.URL.RawPath = app.Path, app.Path, app.Path
	}
	for k, v := range app.ExtraHeader {
		req.Header.Set(k, v)
	}
	if app.Timeout <= 0 {
		app.Timeout = time.Duration(hp.Confer.Opts.Fuse.RequestTimeout) * time.Second
	}
	// set caller unique id
	req.Header.Set(confer.RequestHeaderFromUniqueID, app.AppID)
	t, cancel := context.WithTimeout(req.Context(), app.Timeout)
	defer cancel()
	resp, err := hp.outflow.Proxy.Transport.RoundTrip(req.WithContext(t))
	if err != nil {
		if ignoreErr(err) {
			return nil
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return errno.ErrRequest.Add("request timeout！")
		}
		return errno.ErrResponse.Add(err.Error())
	}
	defer resp.Body.Close()
	// clear header
	for _, h := range confer.HopHeaders {
		resp.Header.Del(h)
	}

	// copy header
	for k, vv := range resp.Header {
		for _, v := range vv {
			ctx.Response().Header().Add(k, v)
		}
	}
	// status code
	ctx.Response().WriteHeader(resp.StatusCode)

	// copy response
	buf := hp.bufferPool.Get()
	_, err = io.CopyBuffer(ctx.Response(), resp.Body, buf)
	hp.bufferPool.Put(buf)
	return err
}

func ignoreErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, io.EOF)
}
