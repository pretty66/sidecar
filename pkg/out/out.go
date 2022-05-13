package out

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strconv"

	"github.com/openmsp/sidecar/pkg/alert"
	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/pkg/errno"
	util "github.com/openmsp/sidecar/utils"

	"github.com/labstack/echo/v4"
	"github.com/openmsp/cilog"
	"github.com/tidwall/gjson"
)

// http type response error
func HTTPJsonOutput(w http.ResponseWriter, code int, out interface{}) error {
	var body []byte
	if out == nil {
		body = []byte(`{"state":1,"msg":"success","data":[]}`)
	} else {
		body = util.JSONEncode(out)
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
	w.WriteHeader(code)
	return nil
}

func HTTPJsonOutputByte(w http.ResponseWriter, code int, out []byte) error {
	if len(out) == 0 {
		out = []byte(`{"state":1,"msg":"success","data":[]}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(out)
	return nil
}

func InflowHTTPJsonRequestError(w http.ResponseWriter, req *http.Request, code int, err error) error {
	var body []byte
	switch eb := err.(type) {
	case *errno.SCError:
		body = eb.Encode()
	default:
		body = errno.ErrRequest.Add(err.Error()).Encode()
		b, _ := httputil.DumpRequest(req, false)
		cilog.LogErrorf(cilog.LogNameDefault, "request err: %v, package: \n[ %s ] ", err, string(b))
	}
	w.Header().Set("Content-Type", "application/json;")
	w.WriteHeader(code)
	_, _ = w.Write(body)
	return nil
}

func HTTPJsonRequestError(w http.ResponseWriter, code int, err error) error {
	var body []byte

	switch eb := err.(type) {
	case *errno.SCError:
		body = eb.Encode()
	default:
		body = errno.ErrRequest.Add(err.Error()).Encode()
		cilog.LogErrorw(cilog.LogNameDefault, "request err ", err)
	}
	w.Header().Set("Content-Type", "application/json;")
	w.WriteHeader(code)
	_, _ = w.Write(body)
	return nil
}

// traffic limiting error response
func HTTPJsonRateLimitError(w http.ResponseWriter, code int, err *errno.SCError) error {
	if err != nil {
		_ = HTTPJsonRequestError(w, code, err)
		alert.RateLimitAlert(confer.Global().Opts.MiscroServiceInfo.UniqueID, confer.Global().Opts.MiscroServiceInfo.Hostname, confer.Global().Opts.MiscroServiceInfo.ServiceName, err)
	}
	return nil
}

// egress response error handling
func HTTPJsonResponseError(ctx echo.Context, proxyArgs *confer.RemoteApp, err error) error {
	errCode := 0
	if err != nil {
		var output []byte
		switch e := err.(type) {
		case *errno.SCError:
			output = e.Encode()
			errCode = e.State
		default:
			output = errno.ErrResponse.Add(err.Error()).Encode()
			errCode = errno.ErrResponse.State
		}
		if ctx.Response().Size == 0 {
			ctx.Response().Header().Set(confer.RequestHeaderContentType, "application/json;")
			ctx.Response().WriteHeader(ctx.Response().Status)
			_, _ = ctx.Response().Write(output)
		}
	}

	if proxyArgs == nil {
		proxyArgs = &confer.RemoteApp{}
	}

	url := ctx.Request().URL.RequestURI()
	if proxyArgs.Scheme != "" && proxyArgs.Host != "" {
		url = proxyArgs.Scheme + "://" + proxyArgs.Host + url
	}

	RequestErrorLogReport("", &RequestErrorLogReportContent{
		TraceID:     util.GetTraceIDNetHTTP(ctx.Request().Header),
		URL:         url,
		ErrCode:     strconv.Itoa(errCode),
		UniqueID:    proxyArgs.AppID,
		Hostname:    proxyArgs.Hostname,
		ContentType: ctx.Request().Header.Get(confer.RequestHeaderContentType),
		Header:      proxyArgs.ExtraHeader,
		ReqBody:     GetRequestBytes(ctx),
		ResBody:     util.GetResponseBody(ctx.Response().Writer),
		Err:         err,
		Status:      ctx.Response().Status,
	})
	return nil
}

func HTTPResponseWaring(ctx echo.Context, proxyArgs *confer.RemoteApp, err *errno.SCError) {
	if proxyArgs == nil {
		proxyArgs = &confer.RemoteApp{}
	}
	url := ctx.Request().URL.RequestURI()
	if proxyArgs.Scheme != "" && proxyArgs.Host != "" {
		url = proxyArgs.Scheme + "://" + proxyArgs.Host + url
	}

	RequestWaringLogReport(&RequestErrorLogReportContent{
		TraceID:     util.GetTraceIDNetHTTP(ctx.Request().Header),
		URL:         url,
		ErrCode:     strconv.Itoa(err.State),
		UniqueID:    proxyArgs.AppID,
		Hostname:    proxyArgs.Hostname,
		ContentType: ctx.Request().Header.Get(confer.RequestHeaderContentType),
		Header:      proxyArgs.ExtraHeader,
		ReqBody:     GetRequestBytes(ctx),
		ResBody:     util.GetResponseBody(ctx.Response().Writer),
	})
}

func HTTPResponseWaringBodySize(ctx echo.Context, proxyArgs *confer.RemoteApp, bodyLen int64) {
	if proxyArgs == nil {
		proxyArgs = &confer.RemoteApp{}
	}
	url := ctx.Request().URL.RequestURI()
	if proxyArgs.Scheme != "" && proxyArgs.Host != "" {
		url = proxyArgs.Scheme + "://" + proxyArgs.Host + url
	}
	body := &RequestErrorLogReportContent{
		TraceID:     util.GetTraceIDNetHTTP(ctx.Request().Header),
		URL:         url,
		UniqueID:    proxyArgs.AppID,
		Hostname:    proxyArgs.Hostname,
		ContentType: ctx.Request().Header.Get(confer.RequestHeaderContentType),
		Header:      proxyArgs.ExtraHeader,
		ReqBody:     GetRequestBytes(ctx),
	}
	body.ReqBodys = string(body.ReqBody)
	if len(body.ReqBody) > 0 && gjson.ValidBytes(body.ReqBody) {
		_ = json.Unmarshal(body.ReqBody, &body.ReqBodys)
	}

	b, err := json.Marshal(body)
	if err != nil {
		cilog.LogErrorw(cilog.LogNameDefault, "ResponseWaringBodySize json.Marshal ", err)
		return
	}
	cilog.LogWarndw(cilog.LogNameDefault, cilog.DynamicFields{
		AppKey:     body.AppKey,
		Channel:    body.Channel,
		SubOrgKey:  body.SubOrgKey,
		TraceID:    body.TraceID,
		Url:        body.URL,
		ErrCode:    body.ErrCode,
		CustomLog1: body.AccountID,
	}, fmt.Sprintf("request response body is too large data size: %d kb ", bodyLen/1024)+string(b))
}

func GetRequestBytes(ctx echo.Context) []byte {
	b, ok := ctx.Get(confer.RequestBodyKey).([]byte)
	if ok {
		return b
	}
	return []byte{}
}
