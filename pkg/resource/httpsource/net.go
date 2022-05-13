package httpsource

import (
	"fmt"
	"time"

	"github.com/openmsp/sidecar/pkg/resource"
)

var ethInterface string

var ErrDefaultEthInterfaceNotfound = fmt.Errorf("default EthInterface notfound")

func (hs *HTTPSource) CurrentNetworkStat(interval time.Duration, callback resource.NetStatCallback) {
	var rxbytesOld, txbytesOld uint64
	var err error
	if ethInterface == "" {
		callback(nil, ErrDefaultEthInterfaceNotfound)
		return
	}
	folder := "/sys/class/net/" + ethInterface + "/statistics/"
	rxbytesOld, err = resource.ReadNumberFromFile(folder + "rx_bytes")
	if err != nil {
		callback(nil, err)
		return
	}
	txbytesOld, err = resource.ReadNumberFromFile(folder + "tx_bytes")
	if err != nil {
		callback(nil, err)
		return
	}
	go func() {
		time.Sleep(interval)
		rxbytesNew, err := resource.ReadNumberFromFile(folder + "rx_bytes")
		if err != nil {
			callback(nil, err)
			return
		}
		txbytesNew, err := resource.ReadNumberFromFile(folder + "tx_bytes")
		if err != nil {
			callback(nil, err)
			return
		}
		stat := &resource.NetworkStat{
			RxBytes: rxbytesNew - rxbytesOld,
			TxBytes: txbytesNew - txbytesOld,
		}
		callback(stat, nil)
	}()
}
