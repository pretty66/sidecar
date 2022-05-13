package metrics

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/pkg/out"
	"github.com/openmsp/sidecar/sc"
	util "github.com/openmsp/sidecar/utils"
	"github.com/openmsp/sidecar/utils/influxdb"

	"github.com/IceFireDB/IceFireDB-Proxy/utils"

	"github.com/labstack/echo/v4"
	"github.com/openmsp/cilog"
	client "github.com/openmsp/kit/influxdata/influxdb1-client/v2"
)

const (
	MspServiceMetricsRedisField = "metrics"
	RequestBodyLimitSize        = 1048576 // 1m
)

type Metrics struct {
	whatData
	conf               confer.InfluxDBConfig
	batchPoints        client.BatchPoints
	point              chan *client.Point
	flushTimer         *time.Ticker
	influxDBHttpClient *influxdb.InfluxDBHttpClient
	configChan         <-chan []byte
}

type metricsData struct {
	UniqueID    string            `json:"unique_id"`
	ServiceName string            `json:"service_name"`
	Host        string            `json:"host"`
	HTTPMethod  string            `json:"http_method"`
	HTTPCode    string            `json:"http_code"`
	Device      string            `json:"device"`
	Unit        string            `json:"unit"`
	What        string            `json:"what"`
	Type        string            `json:"type"`
	Result      interface{}       `json:"result"`
	Stat        string            `json:"stat"`
	BinMax      string            `json:"bin_max"`
	Direction   string            `json:"direction"`
	Mtype       string            `json:"mtype"`
	File        string            `json:"file"`
	Line        string            `json:"line"`
	Env         string            `json:"env"`
	Meta        map[string]string `json:"meta"`
}

type whatData struct {
	UniqueID  string   `json:"unique_id"`
	What      []string `json:"what"`
	Timestamp int64    `json:"timestamp"`
	sync.RWMutex
}

type Response struct {
	State int      `json:"state"`
	Data  struct{} `json:"data"`
	Msg   string   `json:"msg"`
}

func NewMetrics() (metrics *Metrics) {
	bp, err := client.NewBatchPoints(sc.Sc().InfluxDBHttpClient.BatchPointsConfig)
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "custom-influxdb client.NewBatchPoints err", err)
		return
	}
	metrics = &Metrics{
		whatData: whatData{
			UniqueID: confer.Global().Opts.MiscroServiceInfo.UniqueID,
			What:     make([]string, 0),
		},
		conf:               confer.Global().Opts.InfluxDBConf,
		batchPoints:        bp,
		point:              make(chan *client.Point, 16),
		flushTimer:         time.NewTicker(time.Duration(confer.Global().Opts.InfluxDBConf.FlushTime) * time.Second),
		influxDBHttpClient: sc.Sc().InfluxDBHttpClient,
	}
	// go metrics.count()
	metrics.worker()
	metrics.configChan = sc.Sc().RegisterConfigWatcher(MspServiceMetricsRedisField, func(key, value []byte) bool {
		return string(key) == MspServiceMetricsRedisField
	}, 1)
	metrics.listenMetricsWhatsNew()
	return
}

func (mt *Metrics) MetricsHandlerFuncHTTP(ctx echo.Context) error {
	if ctx.Request().ContentLength > RequestBodyLimitSize {
		cilog.LogWarnw(cilog.LogNameSidecar, "Custom timing data body is too large, automatically discarded！")
		return out.HTTPJsonOutput(ctx.Response(), http.StatusOK, &Response{
			State: 1,
			Msg:   "success",
		})
	}
	b, _ := ioutil.ReadAll(ctx.Request().Body)
	res := mt.metricsHandler(ctx.Request().Header.Get("content-Type"), b)
	return out.HTTPJsonOutput(ctx.Response(), http.StatusOK, res)
}

func (mt *Metrics) metricsHandler(contentType string, body []byte) (response *Response) {
	response = &Response{
		State: 1,
		Msg:   "success",
	}
	if len(body) == 0 {
		response.State = -1
		response.Msg = "body is empty"
		return
	}
	switch contentType {
	case "application/json":
		response = mt.handleContentTypeJSON(body)
	case "text/plain":
		response = mt.handleContentTypeText(body)
	default:
		response.State = -1
		response.Msg = "contentType is invalid"
	}
	return
}

func (mt *Metrics) handleContentTypeJSON(body []byte) (response *Response) {
	response = &Response{
		State: 1,
		Msg:   "success",
	}
	var metricsData *metricsData
	err := json.Unmarshal(body, &metricsData)
	if err != nil {
		response.State = -1
		response.Msg = "json body is invalid"
		return
	}
	if !mt.checkWhat(metricsData) {
		response.State = -1
		response.Msg = "the rule is closed or not exists"
		return
	}
	fields := make(map[string]interface{})
	fields["result"] = metricsData.Result
	tags := metricsData.Meta
	pt, err := client.NewPoint(metricsData.What, tags, fields, time.Now())
	if err != nil {
		response.State = -1
		response.Msg = "point error"
		cilog.LogErrorw(cilog.LogNameSidecar, "custom-influxdb client.NewPoint err", err)
		return
	}
	mt.point <- pt
	return
}

func (mt *Metrics) handleContentTypeText(body []byte) (response *Response) {
	response = &Response{
		State: 1,
		Msg:   "success",
	}
	data, err := parseDataText(body)
	if err != nil {
		response.State = -1
		response.Msg = "data is invalid"
		return
	}
	if len(data.UniqueID) == 0 {
		response.State = -1
		response.Msg = "unique_id is required"
		return
	}
	if len(data.What) == 0 {
		response.State = -1
		response.Msg = "what is required"
		return
	}

	if !mt.checkWhat(data) {
		response.State = -1
		response.Msg = "the rule is closed or not exists"
		return
	}

	mt.addPoint(data)
	return
}

func (mt *Metrics) checkWhat(metricsData *metricsData) bool {
	mt.RLock()
	defer mt.RUnlock()
	return util.InArray(metricsData.What, mt.What)
}

func (mt *Metrics) listenMetricsWhatsNew() {
	utils.GoWithRecover(func() {
		for conf := range mt.configChan {
			if len(conf) == 0 && mt.Timestamp == 0 {
				continue
			}

			var whatData *whatData
			if len(conf) != 0 {
				err := json.Unmarshal(conf, &whatData)
				if err != nil {
					cilog.LogErrorw(cilog.LogNameSidecar, "failed to get the custom monitor redis data format："+string(conf), err)
					continue
				}
				if whatData.Timestamp <= mt.Timestamp {
					continue
				}
			}
			mt.UpdateWhatsAndHash(whatData)
			cilog.LogInfow(cilog.LogNameSidecar, "Changes of user-defined monitoring rules are detected and synchronized successfully!!!")
		}
	}, func(r interface{}) {
		time.Sleep(time.Second * 3)
		mt.listenMetricsWhatsNew()
	})
}

func (mt *Metrics) UpdateWhatsAndHash(whatData *whatData) {
	mt.Lock()
	defer mt.Unlock()
	if whatData == nil {
		mt.What = nil
		mt.Timestamp = 0
		return
	}
	if whatData.Timestamp > mt.Timestamp {
		mt.What = whatData.What
		mt.Timestamp = whatData.Timestamp
	}
}

func (mt *Metrics) addPoint(metricsData *metricsData) {
	fields, tags, err := getFieldsAndTags(metricsData)
	if err != nil {
		return
	}
	if len(tags) == 0 {
		return
	}
	pt, err := client.NewPoint(metricsData.What, tags, fields, time.Now())
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "custom-influxdb client.NewPoint err", err)
		return
	}
	mt.point <- pt
}

func getFieldsAndTags(metricsData *metricsData) (fields map[string]interface{}, tags map[string]string, err error) {
	fields = make(map[string]interface{})
	tags = make(map[string]string)
	tagsInterface := make(map[string]interface{})
	result := metricsData.Result

	resultFloat, err := strconv.ParseFloat(result.(string), 64)
	if err == nil {
		fields["result"] = resultFloat
	} else {
		resultInt, e := strconv.Atoi(result.(string))
		if e == nil {
			fields["result"] = resultInt
		}
	}
	if _, ok := fields["result"]; !ok {
		err = errors.New("result is invalid")
		return
	}
	tagsInterfaceByte, _ := json.Marshal(metricsData)
	err = json.Unmarshal(tagsInterfaceByte, &tagsInterface)
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "json.Unmarshal err", err)
		return
	}
	for key, value := range tagsInterface {
		if key == "result" {
			continue
		}
		switch value := value.(type) {
		case string:
			if len(value) > 0 {
				tags[key] = value
			}
		case map[string]interface{}:
			for k, v := range value {
				tags[k] = v.(string)
			}
		}
	}
	if len(metricsData.Meta) > 0 {
		for key, value := range metricsData.Meta {
			tags[key] = value
		}
	}
	return
}

func (mt *Metrics) worker() {
	utils.GoWithRecover(func() {
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
			}
		}
	}, func(r interface{}) {
		time.Sleep(time.Second * 3)
		mt.worker()
	})
}

func (mt *Metrics) flush() {
	mt.Lock()
	defer mt.Unlock()
	if mt.batchPoints.GetPointsNum() == 0 {
		return
	}
	err := mt.influxDBHttpClient.FluxDBHttpWrite(mt.batchPoints)
	defer mt.influxDBHttpClient.FluxDBHttpClose()
	if err != nil {
		if strings.Contains(err.Error(), io.EOF.Error()) {
			err = nil
		} else {
			cilog.LogErrorw(cilog.LogNameSidecar, "custom-influxdb client.Write err", err)
		}
	}

	mt.batchPoints.ClearPoints()
}

func parseDataText(data []byte) (metricsData *metricsData, err error) {
	dataStr := strings.TrimSpace(string(data))
	dataMap := make(map[string]interface{})

	dataSli := strings.Split(dataStr, ",")

	for _, value := range dataSli {
		valueSli := strings.Split(strings.TrimSpace(value), "=")
		if len(valueSli) >= 2 {
			if strings.TrimSpace(valueSli[0]) == "meta" {
				metaMap, e := util.JSONToMap(valueSli[1])
				if e != nil {
					err = e
					cilog.LogErrorw(cilog.LogNameSidecar, "custom-influxdb parseData err", err)
					return nil, err
				}
				dataMap[strings.TrimSpace(valueSli[0])] = metaMap
			} else {
				dataMap[strings.TrimSpace(valueSli[0])] = valueSli[1]
			}
		}
	}
	dataMapByte, _ := json.Marshal(dataMap)
	err = json.Unmarshal(dataMapByte, &metricsData)
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "json.Unmarshal err", err)
	}
	return
}
