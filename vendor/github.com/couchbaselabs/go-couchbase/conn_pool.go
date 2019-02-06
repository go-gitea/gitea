package couchbase

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/couchbase/gomemcached"
	"github.com/couchbase/gomemcached/client"
	"github.com/couchbase/goutils/logging"
)

// GenericMcdAuthHandler is a kind of AuthHandler that performs
// special auth exchange (like non-standard auth, possibly followed by
// select-bucket).
type GenericMcdAuthHandler interface {
	AuthHandler
	AuthenticateMemcachedConn(host string, conn *memcached.Client) error
}

// Error raised when a connection can't be retrieved from a pool.
var TimeoutError = errors.New("timeout waiting to build connection")
var errClosedPool = errors.New("the connection pool is closed")
var errNoPool = errors.New("no connection pool")

// Default timeout for retrieving a connection from the pool.
var ConnPoolTimeout = time.Hour * 24 * 30

// overflow connection closer cycle time
var ConnCloserInterval = time.Second * 30

// ConnPoolAvailWaitTime is the amount of time to wait for an existing
// connection from the pool before considering the creation of a new
// one.
var ConnPoolAvailWaitTime = time.Millisecond

type connectionPool struct {
	host        string
	mkConn      func(host string, ah AuthHandler) (*memcached.Client, error)
	auth        AuthHandler
	connections chan *memcached.Client
	createsem   chan bool
	bailOut     chan bool
	poolSize    int
	connCount   uint64
	inUse       bool
}

func newConnectionPool(host string, ah AuthHandler, closer bool, poolSize, poolOverflow int) *connectionPool {
	connSize := poolSize
	if closer {
		connSize += poolOverflow
	}
	rv := &connectionPool{
		host:        host,
		connections: make(chan *memcached.Client, connSize),
		createsem:   make(chan bool, poolSize+poolOverflow),
		mkConn:      defaultMkConn,
		auth:        ah,
		poolSize:    poolSize,
	}
	if closer {
		rv.bailOut = make(chan bool, 1)
		go rv.connCloser()
	}
	return rv
}

// ConnPoolTimeout is notified whenever connections are acquired from a pool.
var ConnPoolCallback func(host string, source string, start time.Time, err error)

func defaultMkConn(host string, ah AuthHandler) (*memcached.Client, error) {
	var features memcached.Features

	conn, err := memcached.Connect("tcp", host)
	if err != nil {
		return nil, err
	}

	if TCPKeepalive == true {
		conn.SetKeepAliveOptions(time.Duration(TCPKeepaliveInterval) * time.Second)
	}

	if EnableMutationToken == true {
		features = append(features, memcached.FeatureMutationToken)
	}
	if EnableDataType == true {
		features = append(features, memcached.FeatureDataType)
	}

	if EnableXattr == true {
		features = append(features, memcached.FeatureXattr)
	}

	if len(features) > 0 {
		if DefaultTimeout > 0 {
			conn.SetDeadline(getDeadline(noDeadline, DefaultTimeout))
		}

		res, err := conn.EnableFeatures(features)

		if DefaultTimeout > 0 {
			conn.SetDeadline(noDeadline)
		}

		if err != nil && isTimeoutError(err) {
			conn.Close()
			return nil, err
		}

		if err != nil || res.Status != gomemcached.SUCCESS {
			logging.Warnf("Unable to enable features %v", err)
		}
	}

	if gah, ok := ah.(GenericMcdAuthHandler); ok {
		err = gah.AuthenticateMemcachedConn(host, conn)
		if err != nil {
			conn.Close()
			return nil, err
		}
		return conn, nil
	}
	name, pass, bucket := ah.GetCredentials()
	if name != "default" {
		_, err = conn.Auth(name, pass)
		if err != nil {
			conn.Close()
			return nil, err
		}
		// Select bucket (Required for cb_auth creds)
		// Required when doing auth with _admin credentials
		if bucket != "" && bucket != name {
			_, err = conn.SelectBucket(bucket)
			if err != nil {
				conn.Close()
				return nil, err
			}
		}
	}
	return conn, nil
}

func (cp *connectionPool) Close() (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("connectionPool.Close error")
		}
	}()
	if cp.bailOut != nil {

		// defensively, we won't wait if the channel is full
		select {
		case cp.bailOut <- false:
		default:
		}
	}
	close(cp.connections)
	for c := range cp.connections {
		c.Close()
	}
	return
}

func (cp *connectionPool) Node() string {
	return cp.host
}

func (cp *connectionPool) GetWithTimeout(d time.Duration) (rv *memcached.Client, err error) {
	if cp == nil {
		return nil, errNoPool
	}

	path := ""

	if ConnPoolCallback != nil {
		defer func(path *string, start time.Time) {
			ConnPoolCallback(cp.host, *path, start, err)
		}(&path, time.Now())
	}

	path = "short-circuit"

	// short-circuit available connetions.
	select {
	case rv, isopen := <-cp.connections:
		if !isopen {
			return nil, errClosedPool
		}
		atomic.AddUint64(&cp.connCount, 1)
		return rv, nil
	default:
	}

	t := time.NewTimer(ConnPoolAvailWaitTime)
	defer t.Stop()

	// Try to grab an available connection within 1ms
	select {
	case rv, isopen := <-cp.connections:
		path = "avail1"
		if !isopen {
			return nil, errClosedPool
		}
		atomic.AddUint64(&cp.connCount, 1)
		return rv, nil
	case <-t.C:
		// No connection came around in time, let's see
		// whether we can get one or build a new one first.
		t.Reset(d) // Reuse the timer for the full timeout.
		select {
		case rv, isopen := <-cp.connections:
			path = "avail2"
			if !isopen {
				return nil, errClosedPool
			}
			atomic.AddUint64(&cp.connCount, 1)
			return rv, nil
		case cp.createsem <- true:
			path = "create"
			// Build a connection if we can't get a real one.
			// This can potentially be an overflow connection, or
			// a pooled connection.
			rv, err := cp.mkConn(cp.host, cp.auth)
			if err != nil {
				// On error, release our create hold
				<-cp.createsem
			} else {
				atomic.AddUint64(&cp.connCount, 1)
			}
			return rv, err
		case <-t.C:
			return nil, ErrTimeout
		}
	}
}

func (cp *connectionPool) Get() (*memcached.Client, error) {
	return cp.GetWithTimeout(ConnPoolTimeout)
}

func (cp *connectionPool) Return(c *memcached.Client) {
	if c == nil {
		return
	}

	if cp == nil {
		c.Close()
	}

	if c.IsHealthy() {
		defer func() {
			if recover() != nil {
				// This happens when the pool has already been
				// closed and we're trying to return a
				// connection to it anyway.  Just close the
				// connection.
				c.Close()
			}
		}()

		select {
		case cp.connections <- c:
		default:
			<-cp.createsem
			c.Close()
		}
	} else {
		<-cp.createsem
		c.Close()
	}
}

// give the ability to discard a connection from a pool
// useful for ditching connections to the wrong node after a rebalance
func (cp *connectionPool) Discard(c *memcached.Client) {
	<-cp.createsem
	c.Close()
}

// asynchronous connection closer
func (cp *connectionPool) connCloser() {
	var connCount uint64

	t := time.NewTimer(ConnCloserInterval)
	defer t.Stop()

	for {
		connCount = cp.connCount

		// we don't exist anymore! bail out!
		select {
		case <-cp.bailOut:
			return
		case <-t.C:
		}
		t.Reset(ConnCloserInterval)

		// no overflow connections open or sustained requests for connections
		// nothing to do until the next cycle
		if len(cp.connections) <= cp.poolSize ||
			ConnCloserInterval/ConnPoolAvailWaitTime < time.Duration(cp.connCount-connCount) {
			continue
		}

		// close overflow connections now that they are not needed
		for c := range cp.connections {
			select {
			case <-cp.bailOut:
				return
			default:
			}

			// bail out if close did not work out
			if !cp.connCleanup(c) {
				return
			}
			if len(cp.connections) <= cp.poolSize {
				break
			}
		}
	}
}

// close connection with recovery on error
func (cp *connectionPool) connCleanup(c *memcached.Client) (rv bool) {

	// just in case we are closing a connection after
	// bailOut has been sent but we haven't yet read it
	defer func() {
		if recover() != nil {
			rv = false
		}
	}()
	rv = true

	c.Close()
	<-cp.createsem
	return
}

func (cp *connectionPool) StartTapFeed(args *memcached.TapArguments) (*memcached.TapFeed, error) {
	if cp == nil {
		return nil, errNoPool
	}
	mc, err := cp.Get()
	if err != nil {
		return nil, err
	}

	// A connection can't be used after TAP; Dont' count it against the
	// connection pool capacity
	<-cp.createsem

	return mc.StartTapFeed(*args)
}

const DEFAULT_WINDOW_SIZE = 20 * 1024 * 1024 // 20 Mb

func (cp *connectionPool) StartUprFeed(name string, sequence uint32, dcp_buffer_size uint32, data_chan_size int) (*memcached.UprFeed, error) {
	if cp == nil {
		return nil, errNoPool
	}
	mc, err := cp.Get()
	if err != nil {
		return nil, err
	}

	// A connection can't be used after it has been allocated to UPR;
	// Dont' count it against the connection pool capacity
	<-cp.createsem

	uf, err := mc.NewUprFeed()
	if err != nil {
		return nil, err
	}

	if err := uf.UprOpen(name, sequence, dcp_buffer_size); err != nil {
		return nil, err
	}

	if err := uf.StartFeedWithConfig(data_chan_size); err != nil {
		return nil, err
	}

	return uf, nil
}
