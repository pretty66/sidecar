package filesource

import (
	"github.com/openmsp/cilog"
	"github.com/openmsp/kit/go-netstat/netstat"
)

func (*FileSource) GetNetstat() (tcp map[string]int, err error) {
	socks, err := netstat.TCPSocks(netstat.NoopFilter)
	if err != nil {
		cilog.LogErrorw(cilog.LogNameSidecar, "Failed to obtain the service TCP connection. Procedure ", err)
		return
	}
	tcp = make(map[string]int)
	if len(socks) > 0 {
		for _, value := range socks {
			state := value.State.String()
			tcp[state]++
		}
	}
	return
}
