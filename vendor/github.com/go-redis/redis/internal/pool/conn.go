package pool

import (
	"net"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/internal/proto"
)

var noDeadline = time.Time{}

type Conn struct {
	netConn net.Conn

	rd       *proto.Reader
	rdLocked bool
	wr       *proto.Writer

	Inited    bool
	pooled    bool
	createdAt time.Time
	usedAt    atomic.Value
}

func NewConn(netConn net.Conn) *Conn {
	cn := &Conn{
		netConn:   netConn,
		createdAt: time.Now(),
	}
	cn.rd = proto.NewReader(netConn)
	cn.wr = proto.NewWriter(netConn)
	cn.SetUsedAt(time.Now())
	return cn
}

func (cn *Conn) UsedAt() time.Time {
	return cn.usedAt.Load().(time.Time)
}

func (cn *Conn) SetUsedAt(tm time.Time) {
	cn.usedAt.Store(tm)
}

func (cn *Conn) SetNetConn(netConn net.Conn) {
	cn.netConn = netConn
	cn.rd.Reset(netConn)
	cn.wr.Reset(netConn)
}

func (cn *Conn) setReadTimeout(timeout time.Duration) error {
	now := time.Now()
	cn.SetUsedAt(now)
	if timeout > 0 {
		return cn.netConn.SetReadDeadline(now.Add(timeout))
	}
	return cn.netConn.SetReadDeadline(noDeadline)
}

func (cn *Conn) setWriteTimeout(timeout time.Duration) error {
	now := time.Now()
	cn.SetUsedAt(now)
	if timeout > 0 {
		return cn.netConn.SetWriteDeadline(now.Add(timeout))
	}
	return cn.netConn.SetWriteDeadline(noDeadline)
}

func (cn *Conn) Write(b []byte) (int, error) {
	return cn.netConn.Write(b)
}

func (cn *Conn) RemoteAddr() net.Addr {
	return cn.netConn.RemoteAddr()
}

func (cn *Conn) WithReader(timeout time.Duration, fn func(rd *proto.Reader) error) error {
	_ = cn.setReadTimeout(timeout)
	return fn(cn.rd)
}

func (cn *Conn) WithWriter(timeout time.Duration, fn func(wr *proto.Writer) error) error {
	_ = cn.setWriteTimeout(timeout)

	firstErr := fn(cn.wr)
	err := cn.wr.Flush()
	if err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (cn *Conn) Close() error {
	return cn.netConn.Close()
}
