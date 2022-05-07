package sc

import (
	"context"
	"github.com/openmsp/sidecar/pkg/autoredis"
	"github.com/openmsp/sidecar/pkg/confer"
	"github.com/openmsp/sidecar/utils/memorycacher"

	"github.com/openmsp/sesdk/discovery"
)

var _sc *SC

func NewSc(ctx, offlineCtx context.Context, conf *confer.Confer) *SC {
	if _sc != nil {
		return _sc
	}
	_sc = &SC{Ctx: ctx, Confer: conf, OfflineCtx: offlineCtx}
	return _sc
}

func Sc() *SC {
	if _sc == nil {
		panic("sc not init")
	}
	return _sc
}

func Cache() *memorycacher.Cache {
	return Sc().Cache
}

func Discovery() *discovery.Discovery {
	return Sc().Discovery
}

func Redis() autoredis.AutoClient {
	return Sc().RedisCluster
}

func Conf() *confer.Confer {
	return Sc().Confer
}
