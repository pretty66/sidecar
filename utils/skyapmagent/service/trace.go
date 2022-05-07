package service

import (
	"encoding/json"

	nc3 "github.com/SkyAPM/go2sky/reporter/grpc/common"
	nla3 "github.com/SkyAPM/go2sky/reporter/grpc/language-agent"
	log "github.com/openmsp/cilog"
)

type upstreamSegment struct {
	Version int
	segment interface{}
}

type trace struct {
	ApplicationInstance int32    `json:"application_instance"`
	Pid                 int      `json:"pid"`
	ApplicationID       int32    `json:"application_id"`
	UUID                string   `json:"uuid"`
	Version             int      `json:"version"`
	Segment             segment  `json:"segment"`
	GlobalTraceIds      []string `json:"globalTraceIds"`

	// sw8
	TraceID         string `json:"traceId"`
	Service         string `json:"service"`
	ServiceInstance string `json:"serviceInstance"`
}

type segment struct {
	TraceSegmentID string `json:"traceSegmentId"`
	IsSizeLimited  int    `json:"isSizeLimited"`
	Spans          []span `json:"spans"`
}

type span struct {
	Tags          map[string]string `json:"tags"`
	SpanID        int32             `json:"spanId"`
	ParentSpanID  int32             `json:"parentSpanId"`
	StartTime     int64             `json:"startTime"`
	OperationName string            `json:"operationName"`
	Peer          string            `json:"peer"`
	SpanType      int32             `json:"spanType"`
	SpanLayer     int32             `json:"spanLayer"`
	ComponentID   int32             `json:"componentId"`
	ComponentName string            `json:"component"`
	Refs          []ref             `json:"refs"`
	EndTime       int64             `json:"endTime"`
	IsError       int               `json:"isError"`
}

type ref struct {
	Type                        int32  `json:"type"`
	ParentTraceSegmentID        string `json:"parentTraceSegmentId"`
	ParentSpanID                int32  `json:"parentSpanId"`
	ParentApplicationInstanceID int32  `json:"parentApplicationInstanceId"`
	NetworkAddress              string `json:"networkAddress"`
	EntryApplicationInstanceID  int32  `json:"entryApplicationInstanceId"`
	EntryServiceName            string `json:"entryServiceName"`
	ParentServiceName           string `json:"parentServiceName"`

	//	sw8
	TraceID               string `json:"traceId"`
	ParentService         string `json:"parentService"`
	ParentServiceInstance string `json:"parentServiceInstance"`
	ParentEndpoint        string `json:"parentEndpoint"`
	TargetAddress         string `json:"targetAddress"`
}

func (t *Agent) send(segments []*upstreamSegment) {
	for _, segment := range segments {
		if segment == nil || segment.segment == nil {
			continue
		}

		if t.version == 8 {
			t.grpcReport.SendSegment(segment.segment.(*nla3.SegmentObject))
		}
	}
}

func format(version int, j string) (trace, *upstreamSegment) {
	info := trace{}
	err := json.Unmarshal([]byte(j), &info)
	if err != nil {
		log.LogErrorw(log.LogNameSidecar, "apm-agent trace proto decode:", err)
		return info, nil
	}

	if version == 8 {
		var spans []*nla3.SpanObject
		for _, v := range info.Segment.Spans {
			span := &nla3.SpanObject{
				SpanId:        v.SpanID,
				ParentSpanId:  v.ParentSpanID,
				StartTime:     v.StartTime,
				EndTime:       v.EndTime,
				OperationName: v.OperationName,
				Peer:          v.Peer,
				ComponentId:   v.ComponentID,
				IsError:       v.IsError != 0,
			}

			if v.SpanType == 0 {
				span.SpanType = nla3.SpanType_Entry
			} else if v.SpanType == 1 {
				span.SpanType = nla3.SpanType_Exit
			} else if v.SpanType == 2 {
				span.SpanType = nla3.SpanType_Local
			}

			if v.SpanLayer == 3 {
				span.SpanLayer = nla3.SpanLayer_Http
			} else if v.SpanLayer == 1 {
				span.SpanLayer = nla3.SpanLayer_Database
			}

			buildTags3(span, v.Tags)
			buildRefs3(span, v.Refs)

			spans = append(spans, span)
		}

		segmentObject := &nla3.SegmentObject{
			TraceId:         info.TraceID,
			TraceSegmentId:  info.Segment.TraceSegmentID,
			Spans:           spans,
			Service:         info.Service,
			ServiceInstance: info.ServiceInstance,
			IsSizeLimited:   info.Segment.IsSizeLimited != 0,
		}

		return info, &upstreamSegment{
			Version: info.Version,
			segment: segmentObject,
		}
	}
	return info, nil
}

func buildRefs3(span *nla3.SpanObject, refs []ref) {
	// refs
	spanRefs := make([]*nla3.SegmentReference, len(refs))
	for k, rev := range refs {
		var refType nla3.RefType
		if rev.Type == 0 {
			refType = nla3.RefType_CrossProcess
		}
		reference := &nla3.SegmentReference{
			RefType:                  refType,
			TraceId:                  rev.TraceID,
			ParentTraceSegmentId:     rev.ParentTraceSegmentID,
			ParentSpanId:             rev.ParentSpanID,
			ParentService:            rev.ParentService,
			ParentServiceInstance:    rev.ParentServiceInstance,
			ParentEndpoint:           rev.ParentEndpoint,
			NetworkAddressUsedAtPeer: rev.TargetAddress,
		}
		spanRefs[k] = reference
	}

	if len(spanRefs) > 0 {
		span.Refs = spanRefs
	}
}

func buildTags3(span *nla3.SpanObject, t map[string]string) {
	// tags
	tags := make([]*nc3.KeyStringValuePair, len(t))
	i := 0
	for k, v := range t {
		kv := &nc3.KeyStringValuePair{
			Key:   k,
			Value: v,
		}
		tags[i] = kv
		i++
	}

	if len(tags) > 0 {
		span.Tags = tags
	}
}
