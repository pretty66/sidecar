package influxdb

import (
	_ "github.com/openmsp/kit/influxdata/influxdb1-client" // this is important because of the bug in go mod
	client "github.com/openmsp/kit/influxdata/influxdb1-client/v2"
)

// HTTPClient HTTP Client
type HTTPClient struct {
	Client            client.Client
	BatchPointsConfig client.BatchPointsConfig
}

// FluxDBHttpWrite ...
func (p *HTTPClient) FluxDBHttpWrite(bp client.BatchPoints) (err error) {
	return p.Client.Write(bp)
}

// FluxDBHttpClose ...
func (p *HTTPClient) FluxDBHttpClose() (err error) {
	return p.Client.Close()
}
