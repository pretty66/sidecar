package out

import (
	"encoding/json"
	"fmt"

	"github.com/openmsp/cilog"
	"github.com/tidwall/gjson"
)

// request error log msg content
type RequestErrorLogReportContent struct {
	AppKey      string            `json:"-"`
	Channel     string            `json:"-"`
	TraceID     string            `json:"-"`
	SubOrgKey   string            `json:"-"`
	AccountID   string            `json:"-"`
	URL         string            `json:"-"`
	ErrCode     string            `json:"-"`
	UniqueID    string            `json:"unique_id"`
	Hostname    string            `json:"hostname"`
	Token       string            `json:"token"`
	ContentType string            `json:"content_type"`
	Header      map[string]string `json:"header"`
	Status      int               `json:"status"`
	Err         error             `json:"-"`
	ErrMsg      string            `json:"err_msg"`
	ReqBody     []byte            `json:"-"`
	ResBody     []byte            `json:"-"`
	ReqBodys    interface{}       `json:"req_body"`
	ResBodys    interface{}       `json:"res_body"`
}

// a log is reported when a request error occurs
func RequestErrorLogReport(tag string, body *RequestErrorLogReportContent) {
	if len(body.ReqBody) > 4096 {
		body.ReqBody = body.ReqBody[:4096]
	}
	if len(body.ResBody) > 4094 {
		body.ResBody = body.ResBody[:4096]
	}
	body.ReqBodys = string(body.ReqBody)
	body.ResBodys = string(body.ResBody)

	if len(body.ReqBody) > 0 && gjson.ValidBytes(body.ReqBody) {
		_ = json.Unmarshal(body.ReqBody, &body.ReqBodys)
	}

	if len(body.ResBody) > 0 && gjson.ValidBytes(body.ResBody) {
		_ = json.Unmarshal(body.ResBody, &body.ResBodys)
	}
	if body.Err != nil {
		body.ErrMsg = body.Err.Error()
	}
	b, err := json.Marshal(body)
	if err != nil {
		cilog.LogWarnf(cilog.LogNameDefault, "RequestErrorLogReport json.Marshal ", err)
		return
	}
	cilog.LogErrord(cilog.LogNameSidecar, cilog.DynamicFields{
		AppKey:     body.AppKey,
		Channel:    body.Channel,
		SubOrgKey:  body.SubOrgKey,
		TraceID:    body.TraceID,
		Url:        body.URL,
		ErrCode:    body.ErrCode,
		CustomLog1: body.AccountID,
	}, fmt.Sprintf("%sservice request error：%s", tag, string(b)))
}

func RequestWaringLogReport(body *RequestErrorLogReportContent) {
	if len(body.ReqBody) > 4096 {
		body.ReqBody = body.ReqBody[:4096]
	}
	if len(body.ResBody) > 4094 {
		body.ResBody = body.ResBody[:4096]
	}
	body.ReqBodys = string(body.ReqBody)
	body.ResBodys = string(body.ResBody)

	if len(body.ReqBody) > 0 && gjson.ValidBytes(body.ReqBody) {
		_ = json.Unmarshal(body.ReqBody, &body.ReqBodys)
	}

	if len(body.ResBody) > 0 && gjson.ValidBytes(body.ResBody) {
		_ = json.Unmarshal(body.ResBody, &body.ResBodys)
	}

	b, err := json.Marshal(body)
	if err != nil {
		cilog.LogErrorw(cilog.LogNameDefault, "RequestWaringLogReport json.Marshal ", err)
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
	}, "service request exception："+string(b))
}
