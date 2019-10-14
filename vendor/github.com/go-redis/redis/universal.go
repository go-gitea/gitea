package redis

import (
	"crypto/tls"
	"time"
)

// UniversalOptions information is required by UniversalClient to establish
// connections.
type UniversalOptions struct {
	// Either a single address or a seed list of host:port addresses
	// of cluster/sentinel nodes.
	Addrs []string

	// Database to be selected after connecting to the server.
	// Only single-node and failover clients.
	DB int

	// Common options.

	OnConnect          func(*Conn) error
	Password           string
	MaxRetries         int
	MinRetryBackoff    time.Duration
	MaxRetryBackoff    time.Duration
	DialTimeout        time.Duration
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	PoolSize           int
	MinIdleConns       int
	MaxConnAge         time.Duration
	PoolTimeout        time.Duration
	IdleTimeout        time.Duration
	IdleCheckFrequency time.Duration
	TLSConfig          *tls.Config

	// Only cluster clients.

	MaxRedirects   int
	ReadOnly       bool
	RouteByLatency bool
	RouteRandomly  bool

	// The sentinel master name.
	// Only failover clients.
	MasterName string
}

func (o *UniversalOptions) cluster() *ClusterOptions {
	if len(o.Addrs) == 0 {
		o.Addrs = []string{"127.0.0.1:6379"}
	}

	return &ClusterOptions{
		Addrs:     o.Addrs,
		OnConnect: o.OnConnect,

		Password: o.Password,

		MaxRedirects:   o.MaxRedirects,
		ReadOnly:       o.ReadOnly,
		RouteByLatency: o.RouteByLatency,
		RouteRandomly:  o.RouteRandomly,

		MaxRetries:      o.MaxRetries,
		MinRetryBackoff: o.MinRetryBackoff,
		MaxRetryBackoff: o.MaxRetryBackoff,

		DialTimeout:        o.DialTimeout,
		ReadTimeout:        o.ReadTimeout,
		WriteTimeout:       o.WriteTimeout,
		PoolSize:           o.PoolSize,
		MinIdleConns:       o.MinIdleConns,
		MaxConnAge:         o.MaxConnAge,
		PoolTimeout:        o.PoolTimeout,
		IdleTimeout:        o.IdleTimeout,
		IdleCheckFrequency: o.IdleCheckFrequency,

		TLSConfig: o.TLSConfig,
	}
}

func (o *UniversalOptions) failover() *FailoverOptions {
	if len(o.Addrs) == 0 {
		o.Addrs = []string{"127.0.0.1:26379"}
	}

	return &FailoverOptions{
		SentinelAddrs: o.Addrs,
		MasterName:    o.MasterName,
		OnConnect:     o.OnConnect,

		DB:       o.DB,
		Password: o.Password,

		MaxRetries:      o.MaxRetries,
		MinRetryBackoff: o.MinRetryBackoff,
		MaxRetryBackoff: o.MaxRetryBackoff,

		DialTimeout:  o.DialTimeout,
		ReadTimeout:  o.ReadTimeout,
		WriteTimeout: o.WriteTimeout,

		PoolSize:           o.PoolSize,
		MinIdleConns:       o.MinIdleConns,
		MaxConnAge:         o.MaxConnAge,
		PoolTimeout:        o.PoolTimeout,
		IdleTimeout:        o.IdleTimeout,
		IdleCheckFrequency: o.IdleCheckFrequency,

		TLSConfig: o.TLSConfig,
	}
}

func (o *UniversalOptions) simple() *Options {
	addr := "127.0.0.1:6379"
	if len(o.Addrs) > 0 {
		addr = o.Addrs[0]
	}

	return &Options{
		Addr:      addr,
		OnConnect: o.OnConnect,

		DB:       o.DB,
		Password: o.Password,

		MaxRetries:      o.MaxRetries,
		MinRetryBackoff: o.MinRetryBackoff,
		MaxRetryBackoff: o.MaxRetryBackoff,

		DialTimeout:  o.DialTimeout,
		ReadTimeout:  o.ReadTimeout,
		WriteTimeout: o.WriteTimeout,

		PoolSize:           o.PoolSize,
		MinIdleConns:       o.MinIdleConns,
		MaxConnAge:         o.MaxConnAge,
		PoolTimeout:        o.PoolTimeout,
		IdleTimeout:        o.IdleTimeout,
		IdleCheckFrequency: o.IdleCheckFrequency,

		TLSConfig: o.TLSConfig,
	}
}

// --------------------------------------------------------------------

// UniversalClient is an abstract client which - based on the provided options -
// can connect to either clusters, or sentinel-backed failover instances or simple
// single-instance servers. This can be useful for testing cluster-specific
// applications locally.
type UniversalClient interface {
	Cmdable
	Watch(fn func(*Tx) error, keys ...string) error
	Process(cmd Cmder) error
	WrapProcess(fn func(oldProcess func(cmd Cmder) error) func(cmd Cmder) error)
	WrapProcessPipeline(fn func(oldProcess func([]Cmder) error) func([]Cmder) error)
	Subscribe(channels ...string) *PubSub
	PSubscribe(channels ...string) *PubSub
	Close() error
}

var _ UniversalClient = (*Client)(nil)
var _ UniversalClient = (*ClusterClient)(nil)

// NewUniversalClient returns a new multi client. The type of client returned depends
// on the following three conditions:
//
// 1. if a MasterName is passed a sentinel-backed FailoverClient will be returned
// 2. if the number of Addrs is two or more, a ClusterClient will be returned
// 3. otherwise, a single-node redis Client will be returned.
func NewUniversalClient(opts *UniversalOptions) UniversalClient {
	if opts.MasterName != "" {
		return NewFailoverClient(opts.failover())
	} else if len(opts.Addrs) > 1 {
		return NewClusterClient(opts.cluster())
	}
	return NewClient(opts.simple())
}
