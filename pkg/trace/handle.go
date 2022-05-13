package trace

import (
	"context"
	"github.com/openmsp/sidecar/pkg/confer"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/SkyAPM/go2sky"
	v3 "github.com/SkyAPM/go2sky/reporter/grpc/language-agent"
	"github.com/openmsp/cilog"
)

const (
	componentIDGOHttpServer = 5004
	componentIDGOHttpClient = 5005

	TagIdgAppID    = "idg.appid"
	TagIdgAppKey   = "idg.appkey"
	TagIdgChannel  = "idg.channel"
	TagAsyncPubSub = "async.publish"

	spanContextKey = "span"
)

type SkyTrace struct {
	Tracer     *go2sky.Tracer
	inEndpoint string
	m          map[string]*skySpan
	lock       sync.RWMutex
	close      atomic.Value
	grpcReport go2sky.Reporter
}

type skySpan struct {
	entryCtx   context.Context
	idgAppID   string
	idgAppKey  string
	idgChannel string
}

func NewSkyTrace(enable bool, reporter go2sky.Reporter) (t *SkyTrace) {
	t = &SkyTrace{
		m:          make(map[string]*skySpan),
		inEndpoint: confer.Global().Opts.MiscroServiceInfo.PodIP + ":8080",
	}
	t.close.Store(!enable)
	t.grpcReport = reporter
	return
}

func (t *SkyTrace) Init() (err error) {
	t.Tracer, err = go2sky.NewTracer(confer.Global().Opts.MiscroServiceInfo.UniqueID,
		go2sky.WithReporter(t.grpcReport),
	)
	return err
}

func (t *SkyTrace) Reload(enable bool, reporter go2sky.Reporter) {
	if reporter != nil {
		t.grpcReport = reporter
		err := t.Init()
		if err != nil {
			cilog.LogWarnf(cilog.LogNameSidecar, "Reload SkyTrace to open failed err %s", err.Error())
			return
		}
	} else if enable {
		cilog.LogWarnf(cilog.LogNameSidecar, "Reload enable SkyTrace but reporter is nil")
		return
	}

	if enable {
		t.Enable()
	} else {
		t.Disable()
	}
}

func (t *SkyTrace) SetSpanTraceCommonTag(span go2sky.Span, method, uri string, statusCode int) {
	span.SetSpanLayer(v3.SpanLayer_Http) // golang
	span.SetPeer(t.inEndpoint)
	span.SetOperationName(uri)
	span.SetComponent(componentIDGOHttpServer)
	span.Tag(go2sky.TagHTTPMethod, method)
	span.Tag(go2sky.TagURL, uri)
	span.Tag("go_version", runtime.Version())
	span.Tag(go2sky.TagStatusCode, strconv.Itoa(statusCode))
}

func (t *SkyTrace) IsClosed() bool {
	return t.close.Load().(bool)
}

func (t *SkyTrace) Enable() {
	t.close.Store(false)
	cilog.LogInfow(cilog.LogNameSidecar, "Enable the non-inductive full link function on sidecar")
}

func (t *SkyTrace) Disable() {
	t.Close()
}

func (t *SkyTrace) Close() {
	t.close.Store(true)
	t.Tracer = nil
	cilog.LogInfow(cilog.LogNameSidecar, "Disable the non-inductive full link function on sidecar")
}

func setIdgTag2Span(span go2sky.Span, skys *skySpan) {
	if skys.idgAppID != "" {
		span.Tag(TagIdgAppID, skys.idgAppID)
	}
	if skys.idgAppKey != "" {
		span.Tag(TagIdgAppKey, skys.idgAppKey)
	}
	if skys.idgChannel != "" {
		span.Tag(TagIdgChannel, skys.idgChannel)
	}
}
