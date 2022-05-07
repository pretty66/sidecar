package router

import (
	"bytes"
	"context"
	"github.com/openmsp/sidecar/pkg/confer"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/openmsp/cilog"
	"github.com/openmsp/sesdk"
	"github.com/smallnest/weighted"
)

type ShadowRequest struct {
	lock       sync.RWMutex
	TargetList map[string]*weighted.SW // host : PercentSW
}

func NewShadowRequest() *ShadowRequest {
	return &ShadowRequest{}
}

func (s *ShadowRequest) SettingRule(info map[string][]*sesdk.Instance, mirror []Mirror) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.TargetList = map[string]*weighted.SW{}

	for k := range mirror {
		if mirror[k].Percent <= 0 {
			continue
		}
		percent := mirror[k].Percent
		if percent > 100 {
			percent = 100
		}
		host := ""
		if len(mirror[k].Host) > 0 {
			host = mirror[k].Host
		} else {
			host = s.matchInstanceByRoute(info, mirror[k])
		}
		if len(host) == 0 {
			continue
		}
		host = strings.TrimRight(host, "/")
		if _, ok := s.TargetList[host]; ok {
			cilog.LogInfow(cilog.LogNameSidecar, "traffic mirroring matches a duplicate host："+host)
			continue
		}

		s.TargetList[host] = &weighted.SW{}
		s.TargetList[host].Add(true, percent)
		s.TargetList[host].Add(false, 100-percent)
	}
}

func (s *ShadowRequest) matchInstanceByRoute(info map[string][]*sesdk.Instance, mirror Mirror) string {
	for k := range info {
		for index := range info[k] {
			insMetadata := info[k][index].Metadata
			isMatch := true
			for rkey, rvalue := range mirror.Metadata {
				matchVal, ok := insMetadata[rkey]
				if !ok {
					isMatch = false
					break
				}
				if matchVal != rvalue {
					isMatch = false
					break
				}
			}
			if isMatch {
				return info[k][index].Addrs[0]
			}
		}
	}

	return ""
}

func (s *ShadowRequest) NetHTTPShadow(queue chan *http.Request, path string, sourceReq *http.Request) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.TargetList) == 0 {
		return
	}
	reqList := []*http.Request{}

	for host := range s.TargetList {
		req := sourceReq.Clone(context.TODO())
		if !s.TargetList[host].Next().(bool) {
			continue
		}
		for _, h := range confer.HopHeaders {
			req.Header.Del(h)
		}
		remote, err := url.Parse(host)
		if err != nil {
			cilog.LogErrorw(cilog.LogNameSidecar, "The traffic mirroring destination host is incorrect："+host, err)
			continue
		}

		req.URL.Scheme = remote.Scheme
		req.Host, req.URL.Host = remote.Host, remote.Host
		if path != "" {
			req.RequestURI, req.URL.Path, req.URL.RawPath = path, path, path
		}
		req.Header.Set(confer.HTTPRequestTypeKey, confer.HTTPShadowRequest)
		reqList = append(reqList, req)
	}
	var bodyBytes []byte
	if sourceReq.Body != nil {
		bodyBytes, _ = ioutil.ReadAll(sourceReq.Body)
		sourceReq.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	for k := range reqList {
		reqList[k].Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		select {
		case queue <- reqList[k]:
		default:
			cilog.LogWarnw(cilog.LogNameSidecar, "Traffic mirroring failed, and the queue was full. Procedure: "+reqList[k].URL.String())
		}
	}
}
