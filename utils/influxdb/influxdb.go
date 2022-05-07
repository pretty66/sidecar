package influxdb

import (
	"github.com/openmsp/sidecar/pkg/confer"

	"github.com/openmsp/cilog"
	_ "github.com/openmsp/kit/influxdata/influxdb1-client" // this is important because of the bug in go mod
	client "github.com/openmsp/kit/influxdata/influxdb1-client/v2"
)

type InfluxDBUDPClient struct {
	Conf              confer.InfluxDBConfig
	BatchPointsConfig client.BatchPointsConfig
	client            client.Client
}

func (p *InfluxDBUDPClient) newinfluxDBUDPV1Client() *InfluxDBUDPClient {
	udpClient, err := client.NewUDPClient(client.UDPConfig{
		Addr: p.Conf.UDPAddress,
	})
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "InfluxDBUDPClient err", err)
	}
	p.client = udpClient
	return p
}

func (p *InfluxDBUDPClient) FluxDBUDPWrite(bp client.BatchPoints) (err error) {
	err = p.newinfluxDBUDPV1Client().client.Write(bp)
	return
}

type InfluxDBHttpClient struct {
	Client            client.Client
	BatchPointsConfig client.BatchPointsConfig
}

func (p *InfluxDBHttpClient) FluxDBHttpWrite(bp client.BatchPoints) (err error) {
	return p.Client.Write(bp)
}

func (p *InfluxDBHttpClient) FluxDBHttpClose() (err error) {
	return p.Client.Close()
}
