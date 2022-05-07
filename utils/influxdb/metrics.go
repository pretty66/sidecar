package influxdb

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/openmsp/sidecar/pkg/confer"

	"github.com/openmsp/cilog"
	client "github.com/openmsp/kit/influxdata/influxdb1-client/v2"
)

// Metrics ...
type Metrics struct {
	mu                 sync.Mutex
	conf               confer.InfluxDBConfig
	batchPoints        client.BatchPoints
	point              chan *client.Point
	flushTimer         *time.Ticker
	influxDBHttpClient *HTTPClient
	ctx                context.Context
	loggerControl      *loggerControl
}

// MetricsData ...
type MetricsData struct {
	Measurement string                 `json:"measurement"`
	Fields      map[string]interface{} `json:"fields"`
	Tags        map[string]string      `json:"tags"`
}

// Response ...
type Response struct {
	State int      `json:"state"`
	Data  struct{} `json:"data"`
	Msg   string   `json:"msg"`
}

// NewMetrics ...
func NewMetrics(ctx context.Context, influxDBHttpClient *HTTPClient, conf confer.InfluxDBConfig) (metrics *Metrics) {
	if !conf.Enable {
		return &Metrics{
			ctx:  ctx,
			conf: conf,
		}
	}
	bp, err := client.NewBatchPoints(influxDBHttpClient.BatchPointsConfig)
	if err != nil {
		cilog.LogErrorf(cilog.LogNameSidecar, "custom-influxdb client.NewBatchPoints err: %v", err)
		return
	}
	pointCount := conf.FlushSize << 1
	if pointCount < 16 {
		pointCount = 16
	}
	metrics = &Metrics{
		ctx:                ctx,
		conf:               conf,
		batchPoints:        bp,
		point:              make(chan *client.Point, pointCount),
		flushTimer:         time.NewTicker(time.Duration(conf.FlushTime) * time.Second),
		influxDBHttpClient: influxDBHttpClient,
		loggerControl:      NewLoggerControl(time.Millisecond * 100),
	}
	go metrics.worker()
	return
}

func (mt *Metrics) AddPoint(pt *client.Point) {
	if !mt.conf.Enable {
		return
	}
	select {
	case mt.point <- pt:
	default:
		select {
		case <-mt.loggerControl.timer.C:
			mt.loggerControl.Reset()
			cilog.LogWarnf(cilog.LogNameSidecar, "add point to send to influxdb failed name %s", pt.Name())
		default:
		}
	}
}

func (mt *Metrics) AddMetrics(metricsData *MetricsData) {
	if !mt.conf.Enable {
		return
	}
	pt, err := client.NewPoint(metricsData.Measurement, metricsData.Tags, metricsData.Fields, time.Now())
	if err != nil {
		cilog.LogErrorf(cilog.LogNameSidecar, "custom-influxdb client.NewPoint err %v", err)
		return
	}
	mt.point <- pt
}

func (mt *Metrics) worker() {
	for {
		select {
		case p, ok := <-mt.point:
			if !ok {
				mt.flush()
				return
			}
			mt.batchPoints.AddPoint(p)
			if mt.batchPoints.GetPointsNum() >= mt.conf.FlushSize {
				mt.flush()
			}
		case <-mt.flushTimer.C:
			mt.flush()
		case <-mt.ctx.Done():
			mt.flush()
			return
		}
	}
}

func (mt *Metrics) flush() {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if mt.batchPoints.GetPointsNum() == 0 {
		return
	}
	err := mt.influxDBHttpClient.FluxDBHttpWrite(mt.batchPoints)
	defer mt.influxDBHttpClient.FluxDBHttpClose()
	if err != nil {
		if strings.Contains(err.Error(), io.EOF.Error()) {
			err = nil
		} else {
			cilog.LogErrorf(cilog.LogNameSidecar, "custom-influxdb client.Write err %v", err)
		}
	}
	mt.batchPoints.ClearPoints()
}
