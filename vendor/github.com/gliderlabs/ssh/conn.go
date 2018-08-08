package ssh

import (
	"context"
	"net"
	"time"
)

type serverConn struct {
	net.Conn

	idleTimeout   time.Duration
	maxDeadline   time.Time
	closeCanceler context.CancelFunc
}

func (c *serverConn) Write(p []byte) (n int, err error) {
	c.updateDeadline()
	n, err = c.Conn.Write(p)
	if _, isNetErr := err.(net.Error); isNetErr && c.closeCanceler != nil {
		c.closeCanceler()
	}
	return
}

func (c *serverConn) Read(b []byte) (n int, err error) {
	c.updateDeadline()
	n, err = c.Conn.Read(b)
	if _, isNetErr := err.(net.Error); isNetErr && c.closeCanceler != nil {
		c.closeCanceler()
	}
	return
}

func (c *serverConn) Close() (err error) {
	err = c.Conn.Close()
	if c.closeCanceler != nil {
		c.closeCanceler()
	}
	return
}

func (c *serverConn) updateDeadline() {
	switch {
	case c.idleTimeout > 0:
		idleDeadline := time.Now().Add(c.idleTimeout)
		if idleDeadline.Unix() < c.maxDeadline.Unix() {
			c.Conn.SetDeadline(idleDeadline)
			return
		}
		fallthrough
	default:
		c.Conn.SetDeadline(c.maxDeadline)
	}
}
