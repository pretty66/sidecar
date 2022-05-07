package service

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SkyAPM/go2sky"
	log "github.com/openmsp/cilog"
)

type register struct {
	c    net.Conn
	body string
}

// Agent is APM-SideCar
type Agent struct {
	ctx               context.Context
	cancel            context.CancelFunc
	version           int
	sendRate          int
	grpcAddress       string
	socketType        string
	socket            string
	socketListener    net.Listener
	listenerLock      sync.Mutex
	register          chan *register
	registerCache     map[int]registerCache
	registerCacheLock sync.RWMutex
	trace             chan string
	queue             []string
	queueLock         sync.Mutex
	// Shut down the non-inductive full link
	closeSkyTraceFunc func()
	close             atomic.Value
	closeSkyTraceRun  bool
	// The maximum parsing size in bytes
	TraceTrimMaxSize int
	grpcReport       go2sky.Reporter
	MaxConcurrency   chan struct{}
}

// NewAgent is create a apm-agent
func NewAgent(
	ctx context.Context,
	skyVersion int,
	skyCollectorGrpcAddress string,
	agentListenNetType string,
	agentListenNetURI string,
	agentSendRate int,
	traceTrimMaxSize int,
	closeSkyTraceFunc func(),
	grpcReport go2sky.Reporter,
) *Agent {
	if agentSendRate <= 0 {
		agentSendRate = 1
	}
	c, cancel := context.WithCancel(ctx)
	agent := &Agent{
		ctx:               c,
		cancel:            cancel,
		version:           skyVersion,
		grpcAddress:       skyCollectorGrpcAddress,
		socketType:        agentListenNetType,
		socket:            agentListenNetURI,
		sendRate:          agentSendRate,
		register:          make(chan *register),
		trace:             make(chan string),
		registerCache:     make(map[int]registerCache),
		TraceTrimMaxSize:  traceTrimMaxSize * 1024, // 10M
		closeSkyTraceFunc: closeSkyTraceFunc,
		grpcReport:        grpcReport,
		MaxConcurrency:    make(chan struct{}, 2),
	}
	if agent.closeSkyTraceFunc == nil {
		agent.closeSkyTraceFunc = func() {}
	}
	agent.close.Store(false)

	go agent.sub(c)
	return agent
}

// Run is apm-sidecar runner
func (t *Agent) Run() (err error) {
	log.LogInfow(log.LogNameSidecar, "ServiceCar APM agent running.")

	defer func() {
		t.listenerLock.Lock()
		if t.socketListener != nil {
			err = t.socketListener.Close()
			if err != nil {
				log.LogWarnf(log.LogNameSidecar, "APM agent close socket listen err", err.Error())
			}
		}
		t.listenerLock.Unlock()
	}()

	err = t.listenSocket()

	if err != nil {
		return
	}

	return nil
}

func (t *Agent) listenSocket() (err error) {
	fi, _ := os.Stat(t.socket)

	if fi != nil && !fi.Mode().IsDir() {
		if err = os.RemoveAll(t.socket); err != nil {
			log.LogErrorw(log.LogNameSidecar, "apm Agent listenSocket error", err)
			return
		}
	}
	t.listenerLock.Lock()
	t.socketListener, err = net.Listen(t.socketType, t.socket)
	t.listenerLock.Unlock()
	if err != nil {
		log.LogErrorw(log.LogNameSidecar, "apm Agent listen socket error", err)
		return
	}

	err = os.Chmod(t.socket, os.ModeSocket|0o777)
	if err != nil {
		log.LogWarnw(log.LogNameSidecar, err.Error())
	}
	for {
		if t.ctx.Err() != nil {
			return err
		}
		c, er := t.socketListener.Accept()
		if er != nil {
			break
		}
		if !t.closeSkyTraceRun {
			t.closeSkyTraceFunc()
			t.closeSkyTraceRun = true
		}
		conn := NewConn(t, c)
		go conn.Handle()
	}
	return err
}

func (t *Agent) sub(ctx context.Context) {
	heartbeatTicker := time.NewTicker(time.Second * 40)
	defer heartbeatTicker.Stop()
	if t.sendRate < 100 {
		t.sendRate = 100
	}
	traceSendTicker := time.NewTicker(time.Millisecond * time.Duration(t.sendRate))
	defer traceSendTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-traceSendTicker.C:
			l := len(t.queue)
			if l > 0 {
				var segments []*upstreamSegment

				t.queueLock.Lock()
				list := t.queue
				t.queue = []string{}
				t.queueLock.Unlock()

				for _, trace := range list {
					info, st := format(t.version, trace)
					if st != nil {
						t.recoverRegister(info)
						segments = append(segments, st)
					}
				}
				go t.send(segments)
			}
		case <-heartbeatTicker.C:
			if t.close.Load().(bool) {
				return
			}
			go t.heartbeat()
		case register := <-t.register:
			go t.doRegister(register)
		case trace := <-t.trace:
			t.queueLock.Lock()
			if len(t.queue) < 65536*2 {
				t.queue = append(t.queue, trace)
			} else {
				fmt.Println("trace queue is fill.")
			}
			t.queueLock.Unlock()
		}
	}
}

func (t *Agent) Stop() {
	t.close.Store(true)
}

func (t *Agent) Close() {
	t.cancel()
	t.listenerLock.Lock()
	defer t.listenerLock.Unlock()
	if t.socketListener != nil {
		_ = t.socketListener.Close()
	}
}
