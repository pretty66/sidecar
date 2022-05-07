module github.com/openmsp/sidecar

go 1.16

require (
	github.com/IceFireDB/IceFireDB-Proxy v1.0.0
	github.com/SkyAPM/go2sky v0.6.6
	github.com/StackExchange/wmi v0.0.0-20210224194228-fe8f1750fd46 // indirect
	github.com/agrea/ptr v0.0.0-20180711073057-77a518d99b7b
	github.com/buger/jsonparser v1.1.1
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-chassis/go-archaius v1.5.4
	github.com/go-chassis/kie-client v0.1.0
	github.com/go-ole/go-ole v1.2.5 // indirect
	github.com/go-redis/redis/v8 v8.10.0
	github.com/golang/protobuf v1.5.2
	github.com/google/uuid v1.3.0
	github.com/labstack/echo/v4 v4.1.11
	github.com/mitchellh/mapstructure v1.4.1
	github.com/openmsp/cilog v0.0.2
	github.com/openmsp/kit v0.0.2
	github.com/openmsp/sesdk v0.0.0-20220426072140-8441f9bcb308
	github.com/paulbellamy/ratecounter v0.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.1-0.20210607165600-196536534fbb
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	github.com/shirou/gopsutil v3.20.11+incompatible
	github.com/shopspring/decimal v1.2.0
	github.com/sirupsen/logrus v1.8.1
	github.com/smallnest/weighted v0.0.0-20200820100228-10873b4c4c7e
	github.com/sony/gobreaker v0.4.1
	github.com/stretchr/testify v1.7.0
	github.com/tidwall/gjson v1.14.1
	github.com/urfave/cli v1.22.5
	github.com/ztalab/ZACA v0.0.1
	go.uber.org/automaxprocs v1.4.0
	go.uber.org/zap v1.19.1
	golang.org/x/net v0.0.0-20211209124913-491a49abca63 // indirect
	google.golang.org/grpc v1.42.0
	gopkg.in/yaml.v2 v2.4.0
	knative.dev/networking v0.0.0-20211203062838-d65e1ba909fe
	knative.dev/serving v0.27.1
)

replace (
	github.com/IceFireDB/IceFireDB-Proxy => github.com/openmsp/redis-proxy v1.0.1-0.20220426021942-2f9ba7e2db29
	github.com/SkyAPM/go2sky v0.6.6 => github.com/openmsp/go2sky v0.6.1-0.20220425023936-79e1759a2ec3
	github.com/StackExchange/wmi => github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d
	github.com/go-redis/redis/v8 => github.com/openmsp/redis/v8 v8.10.1-0.20210701092452-10bc715a2fea
	github.com/sirupsen/logrus => github.com/sirupsen/logrus v1.6.0
	github.com/ztalab/ZACA => github.com/OpenMSP/ZACA v0.0.0-20220424055415-1c649e7615cc
	google.golang.org/protobuf => google.golang.org/protobuf v1.25.0
)

replace go.etcd.io/etcd => go.etcd.io/etcd v3.3.27+incompatible
