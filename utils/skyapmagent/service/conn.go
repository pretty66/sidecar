package service

import (
	"bytes"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/buger/jsonparser"

	log "github.com/openmsp/cilog"
	cv2 "github.com/openmsp/cilog/v2"
	"github.com/openmsp/sidecar/pkg/confer"
)

type Conn struct {
	agent *Agent
	c     net.Conn
}

func NewConn(a *Agent, c net.Conn) *Conn {
	conn := &Conn{
		agent: a,
		c:     c,
	}

	return conn
}

func (c *Conn) Handle() {
	defer func() {
		c.c.Close()
	}()

	buf := make([]byte, 4096)
	// var json string
	buffer := new(bytes.Buffer)
	var endIndex int
	bigTrace := false
	for {
		n, err := c.c.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.LogWarnw(log.LogNameSidecar, err.Error())
			}
			return
		}
		if c.agent.close.Load().(bool) {
			continue
		}
		buffer.Write(buf[0:n])
		if buffer.Len() > confer.MaxTraceDataSize && !bigTrace {
			select {
			case c.agent.MaxConcurrency <- struct{}{}:
				if confer.MaxTraceDataSize > c.agent.TraceTrimMaxSize {
					log.LogWarnf(log.LogNameSidecar, "apm trace data greater than %dkb, automatically discarded!", confer.MaxTraceDataSize/1024)
					return
				} else if buffer.Len() > c.agent.TraceTrimMaxSize {
					log.LogWarnf(log.LogNameSidecar, "The apm trace data is larger than the pruning upper limit %dkb, and it is automatically discarded!\n", c.agent.TraceTrimMaxSize/1024)
					return
				}
			default:
				log.LogWarnf(log.LogNameSidecar, "The apm trace data is greater than %dkb, and the large trace processing is fully loaded, and it is automatically discarded!", confer.MaxTraceDataSize/1024)
				return
			}
			bigTrace = true
		}
		for {
			endIndex = bytes.IndexByte(buffer.Bytes(), '\n')
			if endIndex >= 0 {
				body := make([]byte, endIndex)
				n, err := buffer.Read(body)
				if n != endIndex || err != nil {
					log.LogInfof(log.LogNameSidecar, "endIndex err read %d length %d", n, endIndex)
					return
				}
				switch string(body[:1]) {
				case "0":
					c.agent.register <- &register{
						c:    c.c,
						body: string(body[1:]),
					}
				case "1":
					if bigTrace {
						needComma := false
						spanBytes := new(bytes.Buffer)
						spanBytes.WriteByte('[')
						traceID, _, _, _ := jsonparser.Get(body[1:], "traceId")
						spans, _, _, err := jsonparser.Get(body[1:], "segment", "spans")
						if err != nil {
							log.LogWarnf(log.LogNameSidecar, "apm trace %s failed to parse span to obtain .segment.span! err: %s", string(traceID), err)
							return
						}
						jsonparser.ArrayEach(spans, func(value []byte, dataType jsonparser.ValueType, offset int, _ error) {
							i, e := jsonparser.GetInt(value, "spanLayer")
							if e == nil && i == 3 {
								if needComma {
									spanBytes.WriteByte(',')
								}
								needComma = true
								spanBytes.Write(value)
							}
						})
						spanBytes.WriteByte(']')
						value, err := jsonparser.Set(body[1:], spanBytes.Bytes(), "segment", "spans")
						if err != nil {
							log.LogWarnf(log.LogNameSidecar, "apm traceId %s failed to parse span and insert .segment.span! err: %s", string(traceID), err)
							return
						}
						cv2.With("traceID", string(traceID)).Infof("apm trace parses a large span, the span size before interception is %d K, and the span size after interception: %d B", endIndex/1024, len(value))
						bodyStr := string(value)
						if !strings.Contains(bodyStr, `"/healthcheck`) {
							c.agent.trace <- bodyStr
						}
					} else {
						bodyStr := string(body[1:])
						if !strings.Contains(bodyStr, `"/healthcheck`) {
							c.agent.trace <- bodyStr
						}
					}

				default:
					log.LogInfof(log.LogNameSidecar, "incorrect format")
					return
				}
				bigTrace = false
				buffer.Read(make([]byte, 1))
				select {
				case <-c.agent.MaxConcurrency:
				default:
				}
			} else {
				break
			}
		}
	}
}

var bodyPool sync.Pool

func init() {
	bodyPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}
}

func (c *Conn) HandleNew() {
	defer func() {
		c.c.Close()
	}()

	buf := make([]byte, 4096)
	json := bodyPool.Get().(*bytes.Buffer)
	defer func() {
		json.Reset()
		bodyPool.Put(json)
	}()
	var endIndex int
	for {
		n, err := c.c.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.LogWarnw(log.LogNameSidecar, err.Error())
			}
			return
		}
		json.Write(buf[:n])
		if json.Len() > confer.MaxTraceDataSize {
			log.LogWarnf(log.LogNameSidecar, "Apm trace data greater than % DKB, automatically discarded!", confer.MaxTraceDataSize/1024)
			return
		}
		for {
			endIndex = bytes.IndexAny(json.Bytes(), "\n")
			if endIndex >= 0 {
				// body := json[0:endIndex]
				body, err := json.ReadString('\n')
				if err != nil {
					log.LogWarnw(log.LogNameSidecar, "Failed to parse data："+err.Error())
					break
				}
				if body[0] == 0x0 {
					c.agent.register <- &register{
						c:    c.c,
						body: body[1:],
					}
				} else if body[0] == 0x1 {
					if !strings.Contains(body[1:], `"/healthcheck`) {
						c.agent.trace <- body[1:]
					}
				}
				// json = json[endIndex+1:]
			} else {
				break
			}
		}
	}
}
