package sidecar

import (
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/openmsp/sidecar/pkg/resource/filesource"
	"github.com/openmsp/sidecar/pkg/resource/httpsource"

	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/pkg/event"
	metrics2 "github.com/openmsp/sidecar/pkg/metrics"
	"github.com/openmsp/sidecar/pkg/resource"
	"github.com/openmsp/sidecar/utils"

	"github.com/openmsp/cilog"
	cv2 "github.com/openmsp/cilog/v2"
	client "github.com/openmsp/kit/influxdata/influxdb1-client/v2"
	"github.com/rcrowley/go-metrics"
	"github.com/shopspring/decimal"
)

// Report microservice attribute information
const (
	RESOURCE_CPU_TYPE  = 1
	RESOURCE_MEM_TYPE  = 2
	RESOURCE_BOTH_TYPE = 3
	// registry bottom alarm
	RC_AGENT_BOTTOM_LINE_ALERT = 1
	// custom alarms for the registry
	RC_AGENT_CUSTOM_ALERT = 2
	// sidecar alarm
	SIDE_CAR_BOTTOM_LINE_ALERT = 3
	// service alarm
	SERVICE_BOTTOM_LINE_ALERT = 4

	alertTypeService   = "service"
	alertTypeSidecar   = "sidecar"
	resourceTypeMemory = "memory"
	resourceTypeCPU    = "cpu"
	resourceTypeBoth   = "both"
)

type ServiceMonitorAttributes struct {
	// k8s node ip
	NodeIP string `json:"node_ip"`
	PodIP  string `json:"pod_ip"`
	// Unique service ID
	UniqueID       string `json:"unique_id"`
	HostName       string `json:"host_name"`
	ServiceName    string `json:"service_name"`
	ServiceVersion string `json:"service_version"`
	AppID          string `json:"app_id"`
	// k8s cluster id
	ClusterID string `json:"cluster_id"`
	SiteUID   string `json:"site_uid"`
	// cluster runtime env
	Env      string `json:"env"`
	MspSeEnv string `json:"msp_se_env"`
	// inlet flow size，Requests per second
	InletFlow int64 `json:"inlet_flow"`
	// outlet flow size，Requests per second
	OutletFlow int64 `json:"outlet_flow"`
	// Incoming traffic Data rate, unit：bytes/s
	RequestBytesRate int `json:"request_bytes_rate"`
	// Egress traffic Data rate, unit：bytes/s
	ResponseBytesRate int `json:"response_bytes_rate"`
	// Traffic received by the nic in POD, in bytessec (regardless of container)
	NetworkRxBytes int64 `json:"network_rx_bytes"`
	// Pod nic output traffic, unit bytessec (regardless of container)
	NetworkTxBytes int64 `json:"network_tx_bytes"`
	// cpu used by sidecar unit：%
	SideCarCPU float64 `json:"side_car_cpu"`
	// sidecar memory used, in units：%
	SideCarMemory float64 `json:"side_car_memory"`
	// memory used by sidecar unit：mb
	SideCarMemoryMB int64 `json:"side_car_memory_mb"`
	// Sidecar Total memory of the current container, expressed in units：mb
	EnvMemoryMB int64 `json:"env_memory_mb"`
	// Sidecar Specifies the memory used by the current container, in units：mb
	EnvMemoryUsedMB int64 `json:"env_memory_used_mb"`
	// Sidecar Specifies the memory used by the current container, in units：%
	EnvMemoryUsed float64 `json:"env_memory_used"`
	// sidecar indicates the number of current cpu cores
	SideCarCPUCoreCount float64 `json:"side_car_cpu_core_count"`
	// sidecar disk read rate，bytes/sec
	SideCarDiskRead int64 `json:"sidecar_disk_read"`
	// sidecar disk write rate，bytes/sec
	SideCarDiskWrite int64 `json:"sidecar_disk_write"`
	// service service disk read rate，bytes/sec
	ServiceDiskRead int64 `json:"service_disk_read"`
	// service service disk write rate，bytes/sec
	ServiceDiskWrite int64 `json:"service_disk_write"`
	// cpu used by service services unit：%
	ServiceCPU float64 `json:"service_cpu"`
	// memory used by a business service in units：%
	ServiceMemory float64 `json:"service_memory"`
	// memory used by a business service in units：mb
	ServiceMemoryMB int64 `json:"service_memory_mb"`
	// the remaining memory of the service in units：mb
	ServiceMemoryMBAV int64 `json:"service_memory_mb_av"`
	// business service cpu core
	ServiceCPUCoreCount float64 `json:"service_cpu_core_count"`
	HealthCheckCode     int     `json:"health_check_code"`
	// health check returns data
	HealthCheckResponse string `json:"health_check_response"`

	// k8s cpu&memory request&limit
	// service memory request unit：M
	ServiceMemRequests int `json:"service_mem_requests"`
	// service memory limit unit：M
	ServiceMemLimits int `json:"service_mem_limits"`
	// service cpu request unit：m
	ServiceCPURequests int `json:"service_cpu_requests"`
	// service cpu limit unit：m
	ServiceCPULimits int `json:"service_cpu_limits"`
	Netstat          map[string]int
	// sidecar cpu、memory
	SidecarMemLimits int   `json:"sidecar_mem_limits"`
	EventTime        int64 `json:"event_time"`
}

// heartbeat attributes report data
type RenewAttribute struct {
	attr map[string]string
	sync.Mutex
}

var resourceDataPool sync.Pool

var renewAttribute = &RenewAttribute{}

func (tp *TrafficProxy) microservicesAttributesHeartbeat() {
	time.Sleep(time.Second * time.Duration(StartSilenceTime))
	log.Printf("Start the microservice attributes heartbeat report, every %d seconds.", tp.Confer.Opts.Discovery.HeartBeatToMsp)

	ticker := time.NewTicker(time.Duration(tp.Confer.Opts.Discovery.HeartBeatToMsp) * time.Second)
	go func() {
		select {
		case <-tp.OfflineCtx.Done():
			ticker.Stop()
		}
	}()
	resourceDataPool.New = func() interface{} {
		return &resourceData{}
	}
	attr := new(ServiceMonitorAttributes)
	cpu := make(chan float64)
	disk := make(chan resource.DiskStat)
	network := make(chan resource.NetworkStat)

	attr.NodeIP = tp.Confer.Opts.MiscroServiceInfo.NodeIP
	attr.PodIP = tp.Confer.Opts.MiscroServiceInfo.PodIP
	attr.UniqueID = tp.Confer.Opts.MiscroServiceInfo.UniqueID
	attr.HostName = tp.Confer.Opts.MiscroServiceInfo.Hostname
	attr.ServiceName = tp.Confer.Opts.MiscroServiceInfo.ServiceName
	attr.AppID = tp.Confer.Opts.MiscroServiceInfo.AppID
	attr.ServiceVersion = tp.Confer.Opts.MiscroServiceInfo.MetaData["version"]
	attr.SiteUID = tp.Confer.Opts.MiscroServiceInfo.Region
	attr.ClusterID = tp.Confer.Opts.MiscroServiceInfo.Zone
	attr.Env = tp.Confer.Opts.MiscroServiceInfo.Env
	attr.MspSeEnv = tp.Confer.Opts.Discovery.Env

	cpuTimes := time.Millisecond * 250
	sideCarResourceHd := filesource.NewFileSource()
	serviceResourceHd := httpsource.NewHttpSource(tp.Confer.Opts.BBR.RemoteResourceURL)
	attr.ServiceMemRequests, _ = strconv.Atoi(tp.Confer.Opts.MiscroServiceInfo.ServiceMemRequests)
	attr.ServiceMemLimits, _ = strconv.Atoi(tp.Confer.Opts.MiscroServiceInfo.ServiceMemLimits)
	attr.ServiceCPURequests, _ = strconv.Atoi(tp.Confer.Opts.MiscroServiceInfo.ServiceCPURequests)
	attr.ServiceCPULimits, _ = strconv.Atoi(tp.Confer.Opts.MiscroServiceInfo.ServiceCPULimits)
	attr.SidecarMemLimits, _ = strconv.Atoi(tp.Confer.Opts.MiscroServiceInfo.SidecarMEMLimits)

	bp, err := client.NewBatchPoints(tp.InfluxDBHttpClient.BatchPointsConfig)
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "influxdb client.NewBatchPoints err", err)
		return
	}
	for range ticker.C {
		var scCPULimit, serverCPULimit float64
		if sideCarResourceHd.InitData() {
			attr.SideCarCPUCoreCount = sideCarResourceHd.GetCPUCount()
		}
		if serviceResourceHd.InitData() {
			attr.ServiceCPUCoreCount = serviceResourceHd.GetCPUCount()
		}
		if attr.SideCarCPUCoreCount > 0 {
			scCPULimit = attr.SideCarCPUCoreCount * tp.Confer.Opts.MSPEventConf.SideCarCPU
		}
		if attr.ServiceCPUCoreCount > 0 {
			serverCPULimit = attr.ServiceCPUCoreCount * tp.Confer.Opts.MSPEventConf.ServerCPU
		}

		attr.EventTime = time.Now().UnixNano() / 1e6
		attr.InletFlow = tp.TrafficInRateCounter.Rate()
		attr.OutletFlow = tp.TrafficOutRateCounter.Rate()

		scRsd := resourceDataPool.Get().(*resourceData)
		scResource := scRsd.getResource(sideCarResourceHd, cpu, cpuTimes, disk, network, false)
		serRsd := resourceDataPool.Get().(*resourceData)
		serverResource := serRsd.getResource(serviceResourceHd, cpu, cpuTimes, disk, network, true)

		attr.EnvMemoryMB = scResource.ContainerTotalMemoryMB
		attr.EnvMemoryUsedMB = scResource.ContainerUsedMemoryMB
		attr.EnvMemoryUsed = scResource.UsedMemoryRatio
		if attr.EnvMemoryMB > 0 {
			attr.SideCarMemory, _ = decimal.NewFromInt(scResource.ProcessUsedMemoryMB).DivRound(decimal.NewFromInt(attr.EnvMemoryMB), 2).Mul(decimal.NewFromInt(100)).Float64()
		} else {
			attr.SideCarMemory = 0
		}
		attr.SideCarCPU = scResource.ContainerCPUUsedRatio
		attr.SideCarMemoryMB = scResource.ProcessUsedMemoryMB
		attr.SideCarDiskRead = int64(scResource.ContainerDiskRead)
		attr.SideCarDiskWrite = int64(scResource.ContainerDiskWrite)
		attr.NetworkRxBytes = int64(scResource.ContainerNetFlowRX)
		attr.NetworkTxBytes = int64(scResource.ContainerNetFlowTX)

		attr.ServiceMemoryMB = serverResource.ContainerUsedMemoryMB
		attr.ServiceMemory = serverResource.UsedMemoryRatio
		attr.ServiceMemoryMBAV = serverResource.ContainerTotalMemoryMB - serverResource.ContainerUsedMemoryMB
		attr.ServiceCPU = serverResource.ContainerCPUUsedRatio
		attr.ServiceDiskRead = int64(serverResource.ContainerDiskRead)
		attr.ServiceDiskWrite = int64(serverResource.ContainerDiskWrite)

		attr.Netstat = scResource.Netstat

		if tp.Confer.Opts.MSPEventConf.Enable {
			tp.reportToRedis(attr, scCPULimit, serverCPULimit)
		}
		if tp.Confer.Opts.InfluxDBConf.Enable {
			tp.reportToInfluxDB(attr, bp)
			bp.ClearPoints()
		}

		scRsd.reset()
		serRsd.reset()
		resourceDataPool.Put(scRsd)
		resourceDataPool.Put(serRsd)
	}
}

type resourceData struct {
	// total container memory unit mb
	ContainerTotalMemoryMB int64
	// memory used by the container in mb
	ContainerUsedMemoryMB int64
	// memory used by the current process
	ProcessUsedMemoryMB int64
	// the percentage of memory that has been used
	UsedMemoryRatio float64
	// cpu usage in a container unit：%
	ContainerCPUUsedRatio float64
	// disk read in container unit：bytes/sec
	ContainerDiskRead uint64
	// write to disk in container unit：bytes/sec
	ContainerDiskWrite uint64
	// traffic received by the nic in pod unit：bytes/sec
	ContainerNetFlowRX uint64
	// output traffic of the nic in pod unit：bytes/sec
	ContainerNetFlowTX uint64
	// TCP connection details
	Netstat map[string]int
}

func (res *resourceData) getResource(rs resource.Resource, cpu chan float64, cpuTimes time.Duration, disk chan resource.DiskStat, network chan resource.NetworkStat, isRemote bool) *resourceData {
	if !rs.InitSuccess() {
		return res
	}
	var err error
	res.ContainerTotalMemoryMB, res.ContainerUsedMemoryMB = utils.GetContainerMemory(rs)
	if res.ContainerTotalMemoryMB == 0 || res.ContainerUsedMemoryMB == 0 {
		return res
	}

	if !isRemote {
		res.ProcessUsedMemoryMB, err = rs.GetRss()
		if err != nil {
			cilog.LogErrorw(cilog.LogNameSidecar, "Failed to obtain the service process memory. Procedure", err)
		}
		// network
		go utils.GetContainerNetwork(rs, network, time.Second)
		networkChan := <-network
		res.ContainerNetFlowRX = networkChan.RxBytes
		res.ContainerNetFlowTX = networkChan.TxBytes
		// TCP
		netstat, err := rs.GetNetstat()
		if err == nil {
			res.Netstat = netstat
		}
	}

	used := decimal.NewFromInt(res.ContainerUsedMemoryMB)
	total := decimal.NewFromInt(res.ContainerTotalMemoryMB)
	res.UsedMemoryRatio, _ = used.DivRound(total, 2).Mul(decimal.NewFromInt(100)).Float64()
	// disk
	go utils.GetContainerDisk(rs, disk, time.Second)
	diskChan := <-disk
	res.ContainerDiskRead = diskChan.Read
	res.ContainerDiskWrite = diskChan.Write
	// cpu
	go utils.GetContainerCPU(rs, cpu, cpuTimes)
	res.ContainerCPUUsedRatio, _ = decimal.NewFromFloat(<-cpu).Round(2).Float64()
	return res
}

func (res *resourceData) reset() {
	res.ContainerCPUUsedRatio = 0
	res.ContainerTotalMemoryMB = 0
	res.ContainerUsedMemoryMB = 0
	res.ProcessUsedMemoryMB = 0
	res.UsedMemoryRatio = 0
	res.ContainerCPUUsedRatio = 0
	res.ContainerDiskRead = 0
	res.ContainerDiskWrite = 0
	res.ContainerNetFlowRX = 0
	res.ContainerNetFlowTX = 0
	for k := range res.Netstat {
		delete(res.Netstat, k)
	}
}

type MSPReportResource struct {
	UniqueID           string  `json:"unique_id"`
	AlertType          int     `json:"alert_type"`
	ResourceType       int     `json:"resource_type"`
	CPU                float64 `json:"cpu"`
	CPUCoreCount       float64 `json:"cpu_core_count"`
	CPUThreshold       float64 `json:"cpu_threshold"`
	Mem                float64 `json:"mem"`
	MemoryMB           int64   `json:"memory_mb"`
	MemThreshold       float64 `json:"mem_threshold"`
	HostName           string  `json:"hostname"`
	ServiceName        string  `json:"service_name"`
	ServiceMemRequests int     `json:"service_mem_requests"`
	ServiceMemLimits   int     `json:"service_mem_limits"`
	ServiceCPURequests int     `json:"service_cpu_requests"`
	ServiceCPULimits   int     `json:"service_cpu_limits"`
	EventTime          int64   `json:"event_time"`
}

func (tp *TrafficProxy) reportToRedis(attr *ServiceMonitorAttributes, scCPULimit, serviceCPULimit float64) {
	if scCPULimit > 0 {
		if attr.EnvMemoryUsed > tp.Confer.Opts.MSPEventConf.SideCarMem || attr.SideCarCPU > scCPULimit {
			resourceType := 0
			if attr.EnvMemoryUsed > tp.Confer.Opts.MSPEventConf.SideCarMem &&
				attr.SideCarCPU > scCPULimit {
				resourceType = 3
			} else if attr.EnvMemoryUsed > tp.Confer.Opts.MSPEventConf.SideCarMem {
				resourceType = 2
			} else if attr.SideCarCPU > scCPULimit {
				resourceType = 1
			}
			data := MSPReportResource{
				UniqueID:     tp.Confer.Opts.MiscroServiceInfo.UniqueID,
				AlertType:    3,
				ResourceType: resourceType,
				CPU:          attr.SideCarCPU,
				CPUCoreCount: attr.SideCarCPUCoreCount,
				CPUThreshold: scCPULimit,
				Mem:          attr.EnvMemoryUsed,
				MemoryMB:     attr.EnvMemoryUsedMB,
				MemThreshold: tp.Confer.Opts.MSPEventConf.SideCarMem,
				HostName:     tp.Confer.Opts.MiscroServiceInfo.Hostname,
				ServiceName:  tp.Confer.Opts.MiscroServiceInfo.ServiceName,
				// cpu&memory request
				ServiceMemRequests: attr.ServiceMemRequests,
				ServiceMemLimits:   attr.ServiceMemLimits,
				ServiceCPURequests: attr.ServiceCPURequests,
				ServiceCPULimits:   attr.ServiceCPULimits,
				EventTime:          time.Now().UnixNano() / 1e6,
			}

			recordResourceWarning(data)
			event.Client().Report(event.EVENT_TYPE_RESOURCE, data)
		}
	}
	if serviceCPULimit > 0 {
		if attr.ServiceMemory > tp.Confer.Opts.MSPEventConf.ServerMem || attr.ServiceCPU > serviceCPULimit {
			resourceType := 0
			if attr.ServiceMemory > tp.Confer.Opts.MSPEventConf.ServerMem && attr.ServiceCPU > serviceCPULimit {
				resourceType = 3
			} else if attr.ServiceMemory > tp.Confer.Opts.MSPEventConf.ServerMem {
				resourceType = 2
			} else if attr.ServiceCPU > serviceCPULimit {
				resourceType = 1
			}

			data := MSPReportResource{
				UniqueID:           tp.Confer.Opts.MiscroServiceInfo.UniqueID,
				AlertType:          4,
				ResourceType:       resourceType,
				CPU:                attr.ServiceCPU,
				CPUCoreCount:       attr.ServiceCPUCoreCount,
				CPUThreshold:       serviceCPULimit,
				Mem:                attr.ServiceMemory,
				MemoryMB:           attr.ServiceMemoryMB,
				MemThreshold:       tp.Confer.Opts.MSPEventConf.ServerMem,
				HostName:           tp.Confer.Opts.MiscroServiceInfo.Hostname,
				ServiceName:        tp.Confer.Opts.MiscroServiceInfo.ServiceName,
				ServiceMemRequests: attr.ServiceMemRequests,
				ServiceMemLimits:   attr.ServiceMemLimits,
				ServiceCPURequests: attr.ServiceCPURequests,
				ServiceCPULimits:   attr.ServiceCPULimits,
				EventTime:          time.Now().UnixNano() / 1e6,
			}
			recordResourceWarning(data)
			event.Client().Report(event.EVENT_TYPE_RESOURCE, data)
		}
	}
}

var (
	_influxDBTags    = make(map[string]string, 20)
	_influxDBFields  = make(map[string]interface{}, 50)
	_regTimestamp    = strconv.FormatInt(time.Now().UnixNano(), 10)
	_httpMetricsTags = make(map[string]string, 20)
)

func (tp *TrafficProxy) reportToInfluxDB(attr *ServiceMonitorAttributes, bp client.BatchPoints) {
	_influxDBTags["app_id"] = attr.AppID
	_influxDBTags["unique_id"] = attr.UniqueID
	_influxDBTags["service_name"] = attr.ServiceName
	_influxDBTags["host_name"] = attr.HostName
	_influxDBTags["node_ip"] = attr.NodeIP
	_influxDBTags["pod_ip"] = attr.PodIP
	_influxDBTags["service_version"] = attr.ServiceVersion
	_influxDBTags["cluster_id"] = attr.ClusterID
	_influxDBTags["site_uid"] = attr.SiteUID
	_influxDBTags["env"] = attr.Env
	_influxDBTags["msp_se_env"] = attr.MspSeEnv
	_influxDBTags["reg_timestamp"] = _regTimestamp

	_influxDBFields["inlet_flow"] = attr.InletFlow
	_influxDBFields["outlet_flow"] = attr.OutletFlow
	_influxDBFields["request_bytes_rate"] = attr.RequestBytesRate
	_influxDBFields["response_bytes_rate"] = attr.ResponseBytesRate
	_influxDBFields["side_car_cpu"] = attr.SideCarCPU
	_influxDBFields["side_car_memory"] = attr.SideCarMemory
	_influxDBFields["side_car_memory_mb"] = attr.SideCarMemoryMB
	_influxDBFields["side_car_env_memory"] = attr.EnvMemoryUsed
	_influxDBFields["side_car_env_memory_mb_av"] = attr.EnvMemoryMB - attr.EnvMemoryUsedMB
	_influxDBFields["side_car_disk_read"] = attr.SideCarDiskRead
	_influxDBFields["side_car_disk_write"] = attr.SideCarDiskWrite
	_influxDBFields["service_cpu"] = attr.ServiceCPU
	_influxDBFields["service_memory"] = attr.ServiceMemory
	_influxDBFields["service_memory_mb"] = attr.ServiceMemoryMB
	_influxDBFields["service_memory_mb_av"] = attr.ServiceMemoryMBAV
	_influxDBFields["service_disk_read"] = attr.ServiceDiskRead
	_influxDBFields["service_disk_write"] = attr.ServiceDiskWrite
	_influxDBFields["health_check_code"] = attr.HealthCheckCode
	_influxDBFields["service_mem_requests"] = attr.ServiceMemRequests
	_influxDBFields["service_mem_limits"] = attr.ServiceMemLimits
	_influxDBFields["service_cpu_requests"] = attr.ServiceCPURequests
	_influxDBFields["service_cpu_limits"] = attr.ServiceCPULimits
	_influxDBFields["network_rx_bytes"] = attr.NetworkRxBytes
	_influxDBFields["network_tx_bytes"] = attr.NetworkTxBytes
	_influxDBFields["sidecar_mem_limits"] = attr.SidecarMemLimits
	_influxDBFields["event_time"] = attr.EventTime

	pt, err := client.NewPoint("monitor_attributes", _influxDBTags, _influxDBFields, time.Now())
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "influxdb client.NewPoint err", err)
		return
	}
	bp.AddPoint(pt)
	if len(attr.Netstat) > 0 {
		fieldsNetstat := make(map[string]interface{})
		for key, value := range attr.Netstat {
			_influxDBTags["tcp_socket_type"] = key
			defer delete(_influxDBTags, "tcp_socket_type")

			fieldsNetstat["tcp_socket"] = value
			ptNetstat, e := client.NewPoint("netstat", _influxDBTags, fieldsNetstat, time.Now())
			if e != nil {
				cilog.LogErrorw(cilog.LogNameSidecar, "influxdb client.NewPoint err", err)
				return
			}
			bp.AddPoint(ptNetstat)
		}
	}
	// http metrics
	_httpMetricsTags["app_id"] = confer.Global().Opts.MiscroServiceInfo.AppID
	_httpMetricsTags["unique_id"] = confer.Global().Opts.MiscroServiceInfo.UniqueID
	_httpMetricsTags["service_name"] = confer.Global().Opts.MiscroServiceInfo.ServiceName
	_httpMetricsTags["host_name"] = confer.Global().Opts.MiscroServiceInfo.Hostname
	_httpMetricsTags["runtime"] = confer.Global().Opts.MiscroServiceInfo.Env
	_httpMetricsTags["version"] = attr.ServiceVersion

	fields := map[string]interface{}{
		metrics2.InflowRequestTotalMetrics: metrics.Get(metrics2.InflowRequestTotalMetrics).(metrics.Counter).Count(),
		metrics2.InflowRequestErrMetrics:   metrics.Get(metrics2.InflowRequestErrMetrics).(metrics.Counter).Count(),
	}
	pt, err = client.NewPoint("monitor_attributes", _httpMetricsTags, fields, time.Now())
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "influxdb client.NewPoint err", err)
		return
	}
	bp.AddPoint(pt)

	err = tp.InfluxDBHttpClient.FluxDBHttpWrite(bp)
	defer tp.InfluxDBHttpClient.FluxDBHttpClose()
	if err != nil {
		if strings.Contains(err.Error(), io.EOF.Error()) {
			err = nil
		} else {
			cilog.LogErrorw(cilog.LogNameSidecar, "influxdb client.Write err", err)
		}
	}
}

func (tp *TrafficProxy) RenewSetAttribute() (out map[string]string) {
	renewAttribute.Lock()
	defer renewAttribute.Unlock()
	if len(renewAttribute.attr) == 0 {
		return
	}
	out = make(map[string]string, len(renewAttribute.attr))
	for k, v := range renewAttribute.attr {
		out[k] = v
	}
	return
}

func recordResourceWarning(alert MSPReportResource) {
	cpu := utils.FloatToString(alert.CPU) + "%"
	cpuThreshold := utils.FloatToString(alert.CPUThreshold) + "%"
	mem := utils.FloatToString(alert.Mem) + "%"
	memThreshold := utils.FloatToString(alert.MemThreshold) + "%"
	var eventMsg, rule, alertType, resourceType string
	switch {
	case alert.AlertType == RC_AGENT_BOTTOM_LINE_ALERT:
		eventMsg = "registry resource bottom alert"
	case alert.AlertType == RC_AGENT_CUSTOM_ALERT:
		eventMsg = "registry resources customize alerts"
	case alert.AlertType == SIDE_CAR_BOTTOM_LINE_ALERT:
		eventMsg = "side car resource bottom of the line warning"
		alertType = alertTypeSidecar
	case alert.AlertType == SERVICE_BOTTOM_LINE_ALERT:
		eventMsg = "micro service resource bottom warning"
		alertType = alertTypeService
	}
	if alert.CPU >= alert.CPUThreshold {
		alert.ResourceType = RESOURCE_CPU_TYPE
		resourceType = resourceTypeCPU
		rule = fmt.Sprintf("current cpu: %s(core), threshold reached: %s(core), k8s set minimum value: %dm, k8s set maximum value: %dm", cpu, cpuThreshold, alert.ServiceCPURequests, alert.ServiceCPULimits)
	}
	if alert.Mem >= alert.MemThreshold {
		alert.ResourceType = RESOURCE_MEM_TYPE
		resourceType = resourceTypeMemory
		rule = fmt.Sprintf("current memory: %s, threshold reached: %s, k8s set minimum value: %dm, k8s set maximum value: %dm", mem, memThreshold, alert.ServiceMemRequests, alert.ServiceMemLimits)
	}
	if alert.CPU >= alert.CPUThreshold && alert.Mem >= alert.MemThreshold {
		alert.ResourceType = RESOURCE_BOTH_TYPE
		resourceType = resourceTypeBoth
		rule = fmt.Sprintf("current cpu: %s(core), threshold reached: %s(core), k8s set minimum value: %dm, k8s set maximum value: %dm; current memory: %s, threshold reached: %s, k8s set minimum value: %dm, k8s set maximum value: %dm",
			cpu,
			cpuThreshold,
			alert.ServiceCPURequests,
			alert.ServiceMemLimits,
			mem,
			memThreshold,
			alert.ServiceMemRequests,
			alert.ServiceMemLimits,
		)
	}
	textLog := fmt.Sprintf(`time of occurrence：%s;event:%s;service name:%s;unique_id:%s;hostname:%s;trigger rule:%s`,
		time.Unix(0, alert.EventTime*int64(time.Millisecond)).Format("2006-01-02 15:04:05.000"),
		eventMsg,
		alert.ServiceName,
		alert.UniqueID,
		alert.HostName,
		rule,
	)
	cv2.With("customLog1", "resource", "customLog2", alertType, "customLog3", resourceType).Error(cilog.LogNameSidecar, textLog)
}
