package confer

import (
	"sync"
)

// Confer is top data struct of confer
type Confer struct {
	Mutex sync.RWMutex
	Opts  scConfS
}

// scConfS is Subordinate configuration
type scConfS struct {
	// ServiceType: sidecar、gateway ...
	ServiceType string `yaml:"service-type"`
	// TrafficInflow Carry ingress request traffic and undertake governance
	TrafficInflow TrafficFlowS `yaml:"traffic_inflow"`
	// TrafficOutflow Carry egress request traffic and undertake governance
	TrafficOutflow TrafficFlowS `yaml:"traffic_outflow"`
	// redis cluster
	RedisClusterConf RedisClusterConf `yaml:"redis-cluster"`
	// memory cache configuration
	MemoryCacheConf MemoryCacheS `yaml:"memory-cache"`
	// debug configuration
	DebugConf DebugConfS `yaml:"debug"`
	// Log configuration
	Log LogConfS `yaml:"log"`
	// Fuse configuration
	Fuse              FuseS          `yaml:"fuse"`
	RateLimiter       RateLimiterS   `yaml:"rate_limiter"`
	Trace             TraceS         `yaml:"trace"`
	BBR               BBRConf        `yaml:"bbr"`
	CoDel             CoDelConf      `yaml:"codel"`
	Discovery         DiscoveryS     `yaml:"discovery"`
	MiscroServiceInfo ServiceInfoS   `yaml:"service_info"`
	Heartbeat         HeartbeatS     `yaml:"heartbeat"`
	K8SHeartbeat      K8SHeartbeatS  `yaml:"k8sheartbeat"`
	InfluxDBConf      InfluxDBConfig `yaml:"influxdb"`
	MSPEventConf      MSPEventS      `yaml:"msp_event"`
	CaConfig          CaConfigS      `yaml:"ca_config"`
	// asynchronous component runtime configuration
	RuntimeConfig RuntimeConfig `yaml:"runtime_config"`
	// remote configuration configuration for sensing
	RemoteConfig RemoteConfig `yaml:"remote_config"`
	// trace intelligent configuration
	AutoTrace AutoTraceS `yaml:"auto_trace"`
	Knative   Knative    `yaml:"knative"` // knative
	RASP      RASP       `yaml:"rasp"`    // rasp
}

type MSPEventS struct {
	Enable     bool    `yaml:"enable"`
	SideCarCPU float64 `yaml:"sidecar_cpu"`
	SideCarMem float64 `yaml:"sidecar_mem"`
	ServerCPU  float64 `yaml:"server_cpu"`
	ServerMem  float64 `yaml:"server_mem"`
}

// RateLimiterS is core Data struct of Miscro Service configuration
type RateLimiterS struct {
	// number of calls in a cycle
	Max int `yaml:"max"`
	// period s is in units
	Duration uint `yaml:"duration"`
	// number of calls in an api cycle
	APIMax int `yaml:"api_max"`
	// the unit of api period is s
	APIDuration uint `yaml:"api_duration"`
	// sensing the msp configuration cycle
	Cycle uint `yaml:"cycle"`
	// redis cluster configuration
	RedisClusterNodes string `yaml:"redis_cluster_nodes"`
	// maximum connection time
	MaxConnAge int `yaml:"max_conn_age"`
	// frequency of idle connection checks
	IdleCheckFrequency int `yaml:"idle_check_frequency"`
}

// ServiceInfoS is core Data struct of Miscro Service configuration
type ServiceInfoS struct {
	Region          string `yaml:"region"`
	Zone            string `yaml:"zone"`
	Env             string `yaml:"env"`
	AppID           string `yaml:"app_id"`
	NodeIP          string `yaml:"node_ip"`
	PodIP           string `yaml:"pod_ip"`
	Namespace       string `yaml:"namespace"`
	Hostname        string `yaml:"hostname"`
	ServiceName     string `yaml:"service_name"`
	EndPointAddress string `yaml:"endpoint_address"`
	UniqueID        string `yaml:"unique_id"`
	HeartBeat       int    `yaml:"heartbeat"`
	// memory corresponding to the service|cpu =>  request&limit
	ServiceMemRequests string `yaml:"service_mem_requests"`
	ServiceMemLimits   string `yaml:"service_mem_limits"`
	ServiceCPURequests string `yaml:"service_cpu_requests"`
	ServiceCPULimits   string `yaml:"service_cpu_limits"`
	// sidecar memory limit in m k8s allocated memory
	SidecarMEMLimits string `yaml:"sidecar_mem_limits"`
	// If sidecar adds metadata data, the MSP display must be synchronized
	MetaData map[string]string `yaml:"meta_data"`
}

// DiscoveryS is core Data struct of discovery configuration
type DiscoveryS struct {
	Address        string `yaml:"address"`          // discovery remote address
	Env            string `yaml:"env"`              // discovery Env
	HeartBeatToMsp int    `yaml:"heartbeat_to_msp"` // report heartbeat cycle to msp
	Scheme         string `yaml:"scheme"`           // access protocol
}

// HeartbeatS is core Data struct of Heartbeat configuration
type HeartbeatS struct {
	Enable bool `yaml:"enable"`
	// default: http://127.0.0.1/heartbeatcheck
	API string `yaml:"api"`
	// health check interval unit: s, default:2s
	Gap uint `yaml:"gap"`
	// timeout period of health check unit: s, default:3s
	Timeout uint `yaml:"timeout"`
	// online after consecutive healthy times, default:3s
	ConsecutiveSuccesses uint32 `yaml:"consecutive_successes"`
	// offline after consecutive failed times
	ConsecutiveFailures uint32 `yaml:"consecutive_failures"`
}

type K8SHeartbeatS struct {
	Enable      bool   `yaml:"enable"`
	API         string `yaml:"api"`
	BindAddress string `yaml:"bind_address"`
}

type AutoTraceS struct {
	// AutoTrace enable
	Enable bool   `yaml:"enable" json:"enable"`
	Type   string `yaml:"type" json:"type"`
	// SkyWalking collector address, only defined in the configuration file
	SkyCollectorGrpcAddress string `yaml:"sky_collector_grpc_address"`
	ConfigOfType            string `yaml:"config_of_type" json:"config_of_type"`
	// last updated
	Timestamp int64 `json:"timestamp"`
}

// FuseS is core Data struct of fuse configuration
type FuseS struct {
	StatusCode         int    `yaml:"status_code"`
	StatusMsg          string `yaml:"status_msg"`
	SerialErrorNumbers uint32 `yaml:"serial_error_numbers"`
	ErrorPercent       uint8  `yaml:"error_percent"`
	MaxRequest         uint32 `yaml:"max_request"`
	Interval           uint32 `yaml:"interval"`
	Timeout            uint32 `yaml:"timeout"`
	RuleType           uint8  `yaml:"rule_type"`
	Target             string `yaml:"target"`
	RequestTimeout     uint32 `yaml:"request_timeout"`
	Cycle              uint32 `yaml:"cycle"`
}

// LogConfS is log configure options
type LogConfS struct {
	OutPut    string `yaml:"output"`
	Debug     bool   `yaml:"debug"`
	LogKey    string `yaml:"key"`
	RedisHost string `yaml:"redis_host"`
	RedisPort int    `yaml:"redis_port"`
}

// TrafficInflowS Carry upstream request traffic and undertake governance work
type TrafficFlowS struct {
	BindProtocolType          string `yaml:"bind_protocol_type"`
	BindNetWork               string `yaml:"bind_network"`
	BindAddress               string `yaml:"bind_address"`
	TargetProtocolType        string `yaml:"target_protocol_type"`
	TargetAddress             string `yaml:"target_address"`
	TargetDialTimeout         int    `yaml:"target_dial_timeout"`
	TargetKeepAlive           int    `yaml:"target_keep_alive"`
	TargetIdleConnTimeout     int    `yaml:"target_idle_conn_timeout"`
	TargetMaxIdleConnsPerHost int    `yaml:"target_max_idle_conns_per_host"`
	EnableWebsocket           bool   `yaml:"enable_websocket"`
}

// RedisClusterConf is redis cluster configure options
type RedisClusterConf struct {
	Enable              bool `yaml:"enable" json:"enable"`
	EnableFaultTolerant bool `yaml:"enable_fault_tolerant" json:"enable_fault_tolerant"`
	// node types：node、cluster；using proxies type = node
	Type          string `yaml:"type" json:"type"`
	UseProxy      bool   `yaml:"use_proxy" json:"use_proxy"`
	ProxyUniqueID string `yaml:"proxy_unique_id" json:"proxy_unique_id"`
	// this configuration is valid when proxies are used
	EnableMTLS bool `yaml:"enable_mtls" json:"enable_mtls"`
	// Cluster start node string, which can contain multiple nodes,
	// separated by commas, multiple nodes are found for high availability
	StartNodes string `yaml:"start_nodes" json:"start_nodes"`
	// Connection timeout parameter of cluster nodes Unit: ms
	ConnTimeOut int `yaml:"conn_timeout" json:"conn_timeout"`
	// Cluster node read timeout parameter Unit: ms
	ConnReadTimeOut int `yaml:"conn_read_timeout" json:"conn_read_timeout"`
	// Cluster node write timeout parameter Unit: ms
	ConnWriteTimeOut int `yaml:"conn_write_timeout" json:"conn_write_timeout"`
	// Cluster node TCP idle survival time Unit: seconds
	ConnAliveTimeOut int `yaml:"conn_alive_timeout" json:"conn_alive_timeout"`
	// The size of the TCP connection pool for each node in the cluster
	ConnPoolSize int `yaml:"conn_pool_size" json:"conn_pool_size"`
	// timed automatic reconnection agent
	TimingSwitchProxy bool `yaml:"timing_switch_proxy" json:"timing_switch_proxy"`
	// timing reconnection period
	SwitchProxyCycle int `yaml:"switch_proxy_cycle" json:"switch_proxy_cycle"`
	// Cluster read-write separation function, the percentage of slave node
	// carrying read traffic: 0-> slave node does not carry any read traffic
	SlaveOperateRate int `yaml:"slave_operate_rate" json:"slave_operate_rate"`
	// Redis cluster status update heartbeat interval:
	// only effective for scenarios where read-write separation is enabled
	ClusterUpdateHeartbeat int `yaml:"cluster_update_heartbeat" json:"cluster_update_heartbeat"`
}

// MemoryCacheS is memory cache configure options
type MemoryCacheS struct {
	Enable bool `yaml:"enable"`
	// The maximum number of items in the cache
	MaxItemsCount int `yaml:"max_items_count"`
	// cache kv default expiration time (unit: ms)
	DefaultExpiration int `yaml:"default_expiration"`
	// cache expiry kv cleanup period (unit: seconds)
	CleanupInterval int `yaml:"cleanup_interval"`
}

// DebugConfS id debug configure options
type DebugConfS struct {
	Enable bool `yaml:"enable"`
	// Middleware performance analysis listening address
	PprofURI string `yaml:"pprof_uri"`
}

// BBRConf is bbr address of the remote resource
type BBRConf struct {
	Enable            bool   `yaml:"enable"`
	RemoteResourceURL string `yaml:"remote_resource_url"`
}

// CoDelConf is configuration of control delay
type CoDelConf struct {
	Enable bool `yaml:"enable"`
}

// InfluxDBConfig
type InfluxDBConfig struct {
	Enable  bool   `yaml:"enable"`
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
	// influxdb udp address of the database，ip:port
	UDPAddress string `yaml:"udp_address"`
	// database name
	Database string `yaml:"database"`
	// precision: n, u, ms, s, m or h
	Precision           string `yaml:"precision"`
	UserName            string `yaml:"username"`
	Password            string `yaml:"password"`
	MaxIdleConns        int    `yaml:"max-idle-conns"`
	MaxIdleConnsPerHost int    `yaml:"max-idle-conns-per-host"`
	IdleConnTimeout     int    `yaml:"idle-conn-timeout"`
	FlushSize           int    `yaml:"flush-size"`
	FlushTime           int    `yaml:"flush-time"`
}

type CaConfigS struct {
	Enable       bool     `yaml:"enable"`
	AuthKey      string   `yaml:"auth_key"`
	Address      string   `yaml:"address"`
	AddressOCSP  string   `yaml:"address_ocsp"`
	Whitelist    []string `yaml:"whitelist"`
	SecurityMode uint8    `yaml:"security_mode"`
}

type RuntimeConfig struct {
	Enable         bool   `yaml:"enable"`
	MetricsCycle   int    `yaml:"metrics_cycle"`
	ComponentsPath string `yaml:"components_path"`
}

type RemoteConfig struct {
	Cycle int `yaml:"cycle"`
}

type TraceS struct {
	// SkyWalking APM enable
	Enable bool `yaml:"enable"`
	// SkyWalking collector address
	SkyCollectorGrpcAddress string `yaml:"sky_collector_grpc_address"`
}

// Knative is the configuration item that controls serverless
type Knative struct {
	// Knative enable
	Enable bool `yaml:"enable" json:"enable"`
	// When the dynamic registry is not available, try directing to the Activator.
	EnableScaleZero bool   `yaml:"enable_scale_zero" json:"enable_scale_zero"`
	MetricsPort     string `yaml:"metrics_port" json:"metrics_port"`
}

// RASP configuration item
type RASP struct {
	Enable           bool   `yaml:"enable" json:"enable"` // RASP enable
	MongoDBAddr      string `yaml:"mongo_db_addr" json:"mongo_db_addr"`
	MongoDBUser      string `yaml:"mongo_db_user" json:"mongo_db_user"`
	MongoDBPwd       string `yaml:"mongo_db_pwd" json:"mongo_db_pwd"`
	MongoDBName      string `yaml:"mongo_db_name" json:"mongo_db_name"`
	MongoDBPoolLimit int    `yaml:"mongo_db_pool_limit" json:"mongo_db_pool_limit"`
}
