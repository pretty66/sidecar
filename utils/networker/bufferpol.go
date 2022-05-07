package networker

import "sync"

type BufferPool struct {
	pool sync.Pool
}

func NewBufferPool() *BufferPool {
	bp := BufferPool{}
	bp.pool.New = func() interface{} {
		b := make([]byte, 32*1024)
		return b
	}
	return &bp
}

func (bp *BufferPool) Get() []byte {
	return bp.pool.Get().([]byte)
}

func (bp *BufferPool) Put(v []byte) {
	bp.pool.Put(v)
}
