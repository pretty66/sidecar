package redisproxy

import "github.com/openmsp/sidecar/utils/bareneter"

func (p *Proxy) accept(conn bareneter.Conn) bool {
	// log.Println(conn.RemoteAddr())
	return true
}

func (p *Proxy) connClosed(conn bareneter.Conn, err error) {
}
