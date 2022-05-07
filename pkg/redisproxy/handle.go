package redisproxy

import (
	"errors"
	"fmt"
	"github.com/openmsp/sidecar/utils/bareneter"
	"io"
	"net"
	"strings"

	"github.com/IceFireDB/IceFireDB-Proxy/pkg/RedSHandle"
	"github.com/IceFireDB/IceFireDB-Proxy/pkg/codis/credis"
	"github.com/IceFireDB/IceFireDB-Proxy/pkg/router"
	"github.com/openmsp/cilog"
)

func (p *Proxy) handle(conn bareneter.Conn) {
	defer func() {
		_ = conn.Close()
		if err := recover(); err != nil {
			cilog.LogErrorw(cilog.LogNameSidecar, "redis proxy panic interception", fmt.Errorf("%v", err))
		}
	}()
	localConn := conn.NetConn()
	localWriteHandle := RedSHandle.NewWriterHandle(localConn)
	decoder := credis.NewDecoderSize(localConn, 1024)
	for {
		resp, err := decoder.Decode()
		if err != nil {
			if err.Error() != io.EOF.Error() && strings.Index(err.Error(), net.ErrClosed.Error()) == -1 {
				cilog.LogErrorw(cilog.LogNameRedis, "description failed to decode resp", err)
			}
			return
		}
		if resp.Type != credis.TypeArray {
			_ = router.WriteError(localWriteHandle, fmt.Errorf(router.ErrUnknownCommand, "cmd"))
			return
		}

		respCount := len(resp.Array)
		if respCount < 1 {
			_ = router.WriteError(localWriteHandle, fmt.Errorf(router.ErrArguments, "cmd"))
			return
		}

		if resp.Array[0].Type != credis.TypeBulkBytes {
			_ = router.WriteError(localWriteHandle, router.ErrCmdTypeWrong)
			return
		}

		commandArgs := make([]interface{}, respCount)
		for i := 0; i < respCount; i++ {
			commandArgs[i] = resp.Array[i].Value
		}
		p.rlock.RLock()
		route := p.router
		p.rlock.RUnlock()
		err = route.Handle(localWriteHandle, commandArgs)
		if err != nil {
			if p.conf.UseProxy {
				select {
				case p.reconn <- struct{}{}:
				default:
				}
			}
			if errors.Is(err, router.ErrLocalWriter) || errors.Is(err, router.ErrLocalFlush) {
				return
			}
			_ = router.WriteError(localWriteHandle, err)
			return
		}
	}
}
