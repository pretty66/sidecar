package influxdb

import (
	"github.com/openmsp/sidecar/pkg/confer"

	"github.com/openmsp/cilog"
	_ "github.com/openmsp/kit/influxdata/influxdb1-client" // this is important because of the bug in go mod
	client "github.com/openmsp/kit/influxdata/influxdb1-client/v2"
)

type UDPClient struct {
	Conf              confer.InfluxDBConfig
	BatchPointsConfig client.BatchPointsConfig
	client            client.Client
}

func (p *UDPClient) newinfluxDBUDPV1Client() *UDPClient {
	udpClient, err := client.NewUDPClient(client.UDPConfig{
		Addr: p.Conf.UDPAddress,
	})
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "UDPClient err", err)
	}
	p.client = udpClient
	return p
}

func (p *UDPClient) FluxDBUDPWrite(bp client.BatchPoints) (err error) {
	err = p.newinfluxDBUDPV1Client().client.Write(bp)
	return
}
