package redis

import (
	"crypto/tls"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/internal"
	"github.com/go-redis/redis/internal/pool"
)

//------------------------------------------------------------------------------

// FailoverOptions are used to configure a failover client and should
// be passed to NewFailoverClient.
type FailoverOptions struct {
	// The master name.
	MasterName string
	// A seed list of host:port addresses of sentinel nodes.
	SentinelAddrs []string

	// Following options are copied from Options struct.

	OnConnect func(*Conn) error

	Password string
	DB       int

	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration

	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	PoolSize           int
	MinIdleConns       int
	MaxConnAge         time.Duration
	PoolTimeout        time.Duration
	IdleTimeout        time.Duration
	IdleCheckFrequency time.Duration

	TLSConfig *tls.Config
}

func (opt *FailoverOptions) options() *Options {
	return &Options{
		Addr: "FailoverClient",

		OnConnect: opt.OnConnect,

		DB:       opt.DB,
		Password: opt.Password,

		MaxRetries: opt.MaxRetries,

		DialTimeout:  opt.DialTimeout,
		ReadTimeout:  opt.ReadTimeout,
		WriteTimeout: opt.WriteTimeout,

		PoolSize:           opt.PoolSize,
		PoolTimeout:        opt.PoolTimeout,
		IdleTimeout:        opt.IdleTimeout,
		IdleCheckFrequency: opt.IdleCheckFrequency,

		TLSConfig: opt.TLSConfig,
	}
}

// NewFailoverClient returns a Redis client that uses Redis Sentinel
// for automatic failover. It's safe for concurrent use by multiple
// goroutines.
func NewFailoverClient(failoverOpt *FailoverOptions) *Client {
	opt := failoverOpt.options()
	opt.init()

	failover := &sentinelFailover{
		masterName:    failoverOpt.MasterName,
		sentinelAddrs: failoverOpt.SentinelAddrs,

		opt: opt,
	}

	c := Client{
		baseClient: baseClient{
			opt:      opt,
			connPool: failover.Pool(),

			onClose: failover.Close,
		},
	}
	c.baseClient.init()
	c.cmdable.setProcessor(c.Process)

	return &c
}

//------------------------------------------------------------------------------

type SentinelClient struct {
	baseClient
}

func NewSentinelClient(opt *Options) *SentinelClient {
	opt.init()
	c := &SentinelClient{
		baseClient: baseClient{
			opt:      opt,
			connPool: newConnPool(opt),
		},
	}
	c.baseClient.init()
	return c
}

func (c *SentinelClient) pubSub() *PubSub {
	pubsub := &PubSub{
		opt: c.opt,

		newConn: func(channels []string) (*pool.Conn, error) {
			return c.newConn()
		},
		closeConn: c.connPool.CloseConn,
	}
	pubsub.init()
	return pubsub
}

// Subscribe subscribes the client to the specified channels.
// Channels can be omitted to create empty subscription.
func (c *SentinelClient) Subscribe(channels ...string) *PubSub {
	pubsub := c.pubSub()
	if len(channels) > 0 {
		_ = pubsub.Subscribe(channels...)
	}
	return pubsub
}

// PSubscribe subscribes the client to the given patterns.
// Patterns can be omitted to create empty subscription.
func (c *SentinelClient) PSubscribe(channels ...string) *PubSub {
	pubsub := c.pubSub()
	if len(channels) > 0 {
		_ = pubsub.PSubscribe(channels...)
	}
	return pubsub
}

func (c *SentinelClient) GetMasterAddrByName(name string) *StringSliceCmd {
	cmd := NewStringSliceCmd("sentinel", "get-master-addr-by-name", name)
	c.Process(cmd)
	return cmd
}

func (c *SentinelClient) Sentinels(name string) *SliceCmd {
	cmd := NewSliceCmd("sentinel", "sentinels", name)
	c.Process(cmd)
	return cmd
}

// Failover forces a failover as if the master was not reachable, and without
// asking for agreement to other Sentinels.
func (c *SentinelClient) Failover(name string) *StatusCmd {
	cmd := NewStatusCmd("sentinel", "failover", name)
	c.Process(cmd)
	return cmd
}

// Reset resets all the masters with matching name. The pattern argument is a
// glob-style pattern. The reset process clears any previous state in a master
// (including a failover in progress), and removes every slave and sentinel
// already discovered and associated with the master.
func (c *SentinelClient) Reset(pattern string) *IntCmd {
	cmd := NewIntCmd("sentinel", "reset", pattern)
	c.Process(cmd)
	return cmd
}

// FlushConfig forces Sentinel to rewrite its configuration on disk, including
// the current Sentinel state.
func (c *SentinelClient) FlushConfig() *StatusCmd {
	cmd := NewStatusCmd("sentinel", "flushconfig")
	c.Process(cmd)
	return cmd
}

// Master shows the state and info of the specified master.
func (c *SentinelClient) Master(name string) *StringStringMapCmd {
	cmd := NewStringStringMapCmd("sentinel", "master", name)
	c.Process(cmd)
	return cmd
}

type sentinelFailover struct {
	sentinelAddrs []string

	opt *Options

	pool     *pool.ConnPool
	poolOnce sync.Once

	mu          sync.RWMutex
	masterName  string
	_masterAddr string
	sentinel    *SentinelClient
	pubsub      *PubSub
}

func (c *sentinelFailover) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sentinel != nil {
		return c.closeSentinel()
	}
	return nil
}

func (c *sentinelFailover) Pool() *pool.ConnPool {
	c.poolOnce.Do(func() {
		c.opt.Dialer = c.dial
		c.pool = newConnPool(c.opt)
	})
	return c.pool
}

func (c *sentinelFailover) dial() (net.Conn, error) {
	addr, err := c.MasterAddr()
	if err != nil {
		return nil, err
	}
	return net.DialTimeout("tcp", addr, c.opt.DialTimeout)
}

func (c *sentinelFailover) MasterAddr() (string, error) {
	addr, err := c.masterAddr()
	if err != nil {
		return "", err
	}
	c.switchMaster(addr)
	return addr, nil
}

func (c *sentinelFailover) masterAddr() (string, error) {
	c.mu.RLock()
	addr := c.getMasterAddr()
	c.mu.RUnlock()
	if addr != "" {
		return addr, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	addr = c.getMasterAddr()
	if addr != "" {
		return addr, nil
	}

	if c.sentinel != nil {
		c.closeSentinel()
	}

	for i, sentinelAddr := range c.sentinelAddrs {
		sentinel := NewSentinelClient(&Options{
			Addr: sentinelAddr,

			MaxRetries: c.opt.MaxRetries,

			DialTimeout:  c.opt.DialTimeout,
			ReadTimeout:  c.opt.ReadTimeout,
			WriteTimeout: c.opt.WriteTimeout,

			PoolSize:           c.opt.PoolSize,
			PoolTimeout:        c.opt.PoolTimeout,
			IdleTimeout:        c.opt.IdleTimeout,
			IdleCheckFrequency: c.opt.IdleCheckFrequency,

			TLSConfig: c.opt.TLSConfig,
		})

		masterAddr, err := sentinel.GetMasterAddrByName(c.masterName).Result()
		if err != nil {
			internal.Logf("sentinel: GetMasterAddrByName master=%q failed: %s",
				c.masterName, err)
			_ = sentinel.Close()
			continue
		}

		// Push working sentinel to the top.
		c.sentinelAddrs[0], c.sentinelAddrs[i] = c.sentinelAddrs[i], c.sentinelAddrs[0]
		c.setSentinel(sentinel)

		addr := net.JoinHostPort(masterAddr[0], masterAddr[1])
		return addr, nil
	}

	return "", errors.New("redis: all sentinels are unreachable")
}

func (c *sentinelFailover) getMasterAddr() string {
	sentinel := c.sentinel

	if sentinel == nil {
		return ""
	}

	addr, err := sentinel.GetMasterAddrByName(c.masterName).Result()
	if err != nil {
		internal.Logf("sentinel: GetMasterAddrByName name=%q failed: %s",
			c.masterName, err)
		return ""
	}

	return net.JoinHostPort(addr[0], addr[1])
}

func (c *sentinelFailover) switchMaster(addr string) {
	c.mu.RLock()
	masterAddr := c._masterAddr
	c.mu.RUnlock()
	if masterAddr == addr {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	internal.Logf("sentinel: new master=%q addr=%q",
		c.masterName, addr)
	_ = c.Pool().Filter(func(cn *pool.Conn) bool {
		return cn.RemoteAddr().String() != addr
	})
	c._masterAddr = addr
}

func (c *sentinelFailover) setSentinel(sentinel *SentinelClient) {
	c.discoverSentinels(sentinel)
	c.sentinel = sentinel

	c.pubsub = sentinel.Subscribe("+switch-master")
	go c.listen(c.pubsub)
}

func (c *sentinelFailover) closeSentinel() error {
	var firstErr error

	err := c.pubsub.Close()
	if err != nil && firstErr == err {
		firstErr = err
	}
	c.pubsub = nil

	err = c.sentinel.Close()
	if err != nil && firstErr == err {
		firstErr = err
	}
	c.sentinel = nil

	return firstErr
}

func (c *sentinelFailover) discoverSentinels(sentinel *SentinelClient) {
	sentinels, err := sentinel.Sentinels(c.masterName).Result()
	if err != nil {
		internal.Logf("sentinel: Sentinels master=%q failed: %s", c.masterName, err)
		return
	}
	for _, sentinel := range sentinels {
		vals := sentinel.([]interface{})
		for i := 0; i < len(vals); i += 2 {
			key := vals[i].(string)
			if key == "name" {
				sentinelAddr := vals[i+1].(string)
				if !contains(c.sentinelAddrs, sentinelAddr) {
					internal.Logf("sentinel: discovered new sentinel=%q for master=%q",
						sentinelAddr, c.masterName)
					c.sentinelAddrs = append(c.sentinelAddrs, sentinelAddr)
				}
			}
		}
	}
}

func (c *sentinelFailover) listen(pubsub *PubSub) {
	ch := pubsub.Channel()
	for {
		msg, ok := <-ch
		if !ok {
			break
		}

		if msg.Channel == "+switch-master" {
			parts := strings.Split(msg.Payload, " ")
			if parts[0] != c.masterName {
				internal.Logf("sentinel: ignore addr for master=%q", parts[0])
				continue
			}
			addr := net.JoinHostPort(parts[3], parts[4])
			c.switchMaster(addr)
		}
	}
}

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
