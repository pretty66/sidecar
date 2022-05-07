package bareneter

import (
	"net"
)

type conn struct {
	conn   net.Conn
	addr   string
	ctx    interface{}
	closed bool
}

// Conn represents a client connection
type Conn interface {
	// RemoteAddr returns the remote address of the client connection.
	RemoteAddr() string

	// Close closes the connection.
	Close() error
	Context() interface{}

	// SetContext sets a user-defined context
	SetContext(v interface{})
	NetConn() net.Conn
}

func (c *conn) Context() interface{} { return c.ctx }

func (c *conn) SetContext(v interface{}) { c.ctx = v }

func (c *conn) RemoteAddr() string { return c.addr }

func (c *conn) NetConn() net.Conn {
	return c.conn
}

func (c *conn) Close() error {
	c.closed = true
	return c.conn.Close()
}
