package redis

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/internal"
	"github.com/go-redis/redis/internal/consistenthash"
	"github.com/go-redis/redis/internal/hashtag"
	"github.com/go-redis/redis/internal/pool"
)

// Hash is type of hash function used in consistent hash.
type Hash consistenthash.Hash

var errRingShardsDown = errors.New("redis: all ring shards are down")

// RingOptions are used to configure a ring client and should be
// passed to NewRing.
type RingOptions struct {
	// Map of name => host:port addresses of ring shards.
	Addrs map[string]string

	// Frequency of PING commands sent to check shards availability.
	// Shard is considered down after 3 subsequent failed checks.
	HeartbeatFrequency time.Duration

	// Hash function used in consistent hash.
	// Default is crc32.ChecksumIEEE.
	Hash Hash

	// Number of replicas in consistent hash.
	// Default is 100 replicas.
	//
	// Higher number of replicas will provide less deviation, that is keys will be
	// distributed to nodes more evenly.
	//
	// Following is deviation for common nreplicas:
	//  --------------------------------------------------------
	//  | nreplicas | standard error | 99% confidence interval |
	//  |     10    |     0.3152     |      (0.37, 1.98)       |
	//  |    100    |     0.0997     |      (0.76, 1.28)       |
	//  |   1000    |     0.0316     |      (0.92, 1.09)       |
	//  --------------------------------------------------------
	//
	//  See https://arxiv.org/abs/1406.2294 for reference
	HashReplicas int

	// Following options are copied from Options struct.

	OnConnect func(*Conn) error

	DB       int
	Password string

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
}

func (opt *RingOptions) init() {
	if opt.HeartbeatFrequency == 0 {
		opt.HeartbeatFrequency = 500 * time.Millisecond
	}

	if opt.HashReplicas == 0 {
		opt.HashReplicas = 100
	}

	switch opt.MinRetryBackoff {
	case -1:
		opt.MinRetryBackoff = 0
	case 0:
		opt.MinRetryBackoff = 8 * time.Millisecond
	}
	switch opt.MaxRetryBackoff {
	case -1:
		opt.MaxRetryBackoff = 0
	case 0:
		opt.MaxRetryBackoff = 512 * time.Millisecond
	}
}

func (opt *RingOptions) clientOptions() *Options {
	return &Options{
		OnConnect: opt.OnConnect,

		DB:       opt.DB,
		Password: opt.Password,

		DialTimeout:  opt.DialTimeout,
		ReadTimeout:  opt.ReadTimeout,
		WriteTimeout: opt.WriteTimeout,

		PoolSize:           opt.PoolSize,
		MinIdleConns:       opt.MinIdleConns,
		MaxConnAge:         opt.MaxConnAge,
		PoolTimeout:        opt.PoolTimeout,
		IdleTimeout:        opt.IdleTimeout,
		IdleCheckFrequency: opt.IdleCheckFrequency,
	}
}

//------------------------------------------------------------------------------

type ringShard struct {
	Client *Client
	down   int32
}

func (shard *ringShard) String() string {
	var state string
	if shard.IsUp() {
		state = "up"
	} else {
		state = "down"
	}
	return fmt.Sprintf("%s is %s", shard.Client, state)
}

func (shard *ringShard) IsDown() bool {
	const threshold = 3
	return atomic.LoadInt32(&shard.down) >= threshold
}

func (shard *ringShard) IsUp() bool {
	return !shard.IsDown()
}

// Vote votes to set shard state and returns true if state was changed.
func (shard *ringShard) Vote(up bool) bool {
	if up {
		changed := shard.IsDown()
		atomic.StoreInt32(&shard.down, 0)
		return changed
	}

	if shard.IsDown() {
		return false
	}

	atomic.AddInt32(&shard.down, 1)
	return shard.IsDown()
}

//------------------------------------------------------------------------------

type ringShards struct {
	opt *RingOptions

	mu     sync.RWMutex
	hash   *consistenthash.Map
	shards map[string]*ringShard // read only
	list   []*ringShard          // read only
	len    int
	closed bool
}

func newRingShards(opt *RingOptions) *ringShards {
	return &ringShards{
		opt: opt,

		hash:   newConsistentHash(opt),
		shards: make(map[string]*ringShard),
	}
}

func (c *ringShards) Add(name string, cl *Client) {
	shard := &ringShard{Client: cl}
	c.hash.Add(name)
	c.shards[name] = shard
	c.list = append(c.list, shard)
}

func (c *ringShards) List() []*ringShard {
	c.mu.RLock()
	list := c.list
	c.mu.RUnlock()
	return list
}

func (c *ringShards) Hash(key string) string {
	c.mu.RLock()
	hash := c.hash.Get(key)
	c.mu.RUnlock()
	return hash
}

func (c *ringShards) GetByKey(key string) (*ringShard, error) {
	key = hashtag.Key(key)

	c.mu.RLock()

	if c.closed {
		c.mu.RUnlock()
		return nil, pool.ErrClosed
	}

	hash := c.hash.Get(key)
	if hash == "" {
		c.mu.RUnlock()
		return nil, errRingShardsDown
	}

	shard := c.shards[hash]
	c.mu.RUnlock()

	return shard, nil
}

func (c *ringShards) GetByHash(name string) (*ringShard, error) {
	if name == "" {
		return c.Random()
	}

	c.mu.RLock()
	shard := c.shards[name]
	c.mu.RUnlock()
	return shard, nil
}

func (c *ringShards) Random() (*ringShard, error) {
	return c.GetByKey(strconv.Itoa(rand.Int()))
}

// heartbeat monitors state of each shard in the ring.
func (c *ringShards) Heartbeat(frequency time.Duration) {
	ticker := time.NewTicker(frequency)
	defer ticker.Stop()
	for range ticker.C {
		var rebalance bool

		c.mu.RLock()

		if c.closed {
			c.mu.RUnlock()
			break
		}

		shards := c.list
		c.mu.RUnlock()

		for _, shard := range shards {
			err := shard.Client.Ping().Err()
			if shard.Vote(err == nil || err == pool.ErrPoolTimeout) {
				internal.Logf("ring shard state changed: %s", shard)
				rebalance = true
			}
		}

		if rebalance {
			c.rebalance()
		}
	}
}

// rebalance removes dead shards from the Ring.
func (c *ringShards) rebalance() {
	c.mu.RLock()
	shards := c.shards
	c.mu.RUnlock()

	hash := newConsistentHash(c.opt)
	var shardsNum int
	for name, shard := range shards {
		if shard.IsUp() {
			hash.Add(name)
			shardsNum++
		}
	}

	c.mu.Lock()
	c.hash = hash
	c.len = shardsNum
	c.mu.Unlock()
}

func (c *ringShards) Len() int {
	c.mu.RLock()
	l := c.len
	c.mu.RUnlock()
	return l
}

func (c *ringShards) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	var firstErr error
	for _, shard := range c.shards {
		if err := shard.Client.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	c.hash = nil
	c.shards = nil
	c.list = nil

	return firstErr
}

//------------------------------------------------------------------------------

// Ring is a Redis client that uses consistent hashing to distribute
// keys across multiple Redis servers (shards). It's safe for
// concurrent use by multiple goroutines.
//
// Ring monitors the state of each shard and removes dead shards from
// the ring. When a shard comes online it is added back to the ring. This
// gives you maximum availability and partition tolerance, but no
// consistency between different shards or even clients. Each client
// uses shards that are available to the client and does not do any
// coordination when shard state is changed.
//
// Ring should be used when you need multiple Redis servers for caching
// and can tolerate losing data when one of the servers dies.
// Otherwise you should use Redis Cluster.
type Ring struct {
	cmdable

	ctx context.Context

	opt           *RingOptions
	shards        *ringShards
	cmdsInfoCache *cmdsInfoCache

	process         func(Cmder) error
	processPipeline func([]Cmder) error
}

func NewRing(opt *RingOptions) *Ring {
	opt.init()

	ring := &Ring{
		opt:    opt,
		shards: newRingShards(opt),
	}
	ring.cmdsInfoCache = newCmdsInfoCache(ring.cmdsInfo)

	ring.process = ring.defaultProcess
	ring.processPipeline = ring.defaultProcessPipeline

	ring.init()

	for name, addr := range opt.Addrs {
		clopt := opt.clientOptions()
		clopt.Addr = addr
		ring.shards.Add(name, NewClient(clopt))
	}

	go ring.shards.Heartbeat(opt.HeartbeatFrequency)

	return ring
}

func (c *Ring) init() {
	c.cmdable.setProcessor(c.Process)
}

func (c *Ring) Context() context.Context {
	if c.ctx != nil {
		return c.ctx
	}
	return context.Background()
}

func (c *Ring) WithContext(ctx context.Context) *Ring {
	if ctx == nil {
		panic("nil context")
	}
	c2 := c.clone()
	c2.ctx = ctx
	return c2
}

func (c *Ring) clone() *Ring {
	cp := *c
	cp.init()

	return &cp
}

// Options returns read-only Options that were used to create the client.
func (c *Ring) Options() *RingOptions {
	return c.opt
}

func (c *Ring) retryBackoff(attempt int) time.Duration {
	return internal.RetryBackoff(attempt, c.opt.MinRetryBackoff, c.opt.MaxRetryBackoff)
}

// PoolStats returns accumulated connection pool stats.
func (c *Ring) PoolStats() *PoolStats {
	shards := c.shards.List()
	var acc PoolStats
	for _, shard := range shards {
		s := shard.Client.connPool.Stats()
		acc.Hits += s.Hits
		acc.Misses += s.Misses
		acc.Timeouts += s.Timeouts
		acc.TotalConns += s.TotalConns
		acc.IdleConns += s.IdleConns
	}
	return &acc
}

// Len returns the current number of shards in the ring.
func (c *Ring) Len() int {
	return c.shards.Len()
}

// Subscribe subscribes the client to the specified channels.
func (c *Ring) Subscribe(channels ...string) *PubSub {
	if len(channels) == 0 {
		panic("at least one channel is required")
	}

	shard, err := c.shards.GetByKey(channels[0])
	if err != nil {
		// TODO: return PubSub with sticky error
		panic(err)
	}
	return shard.Client.Subscribe(channels...)
}

// PSubscribe subscribes the client to the given patterns.
func (c *Ring) PSubscribe(channels ...string) *PubSub {
	if len(channels) == 0 {
		panic("at least one channel is required")
	}

	shard, err := c.shards.GetByKey(channels[0])
	if err != nil {
		// TODO: return PubSub with sticky error
		panic(err)
	}
	return shard.Client.PSubscribe(channels...)
}

// ForEachShard concurrently calls the fn on each live shard in the ring.
// It returns the first error if any.
func (c *Ring) ForEachShard(fn func(client *Client) error) error {
	shards := c.shards.List()
	var wg sync.WaitGroup
	errCh := make(chan error, 1)
	for _, shard := range shards {
		if shard.IsDown() {
			continue
		}

		wg.Add(1)
		go func(shard *ringShard) {
			defer wg.Done()
			err := fn(shard.Client)
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
			}
		}(shard)
	}
	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (c *Ring) cmdsInfo() (map[string]*CommandInfo, error) {
	shards := c.shards.List()
	firstErr := errRingShardsDown
	for _, shard := range shards {
		cmdsInfo, err := shard.Client.Command().Result()
		if err == nil {
			return cmdsInfo, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, firstErr
}

func (c *Ring) cmdInfo(name string) *CommandInfo {
	cmdsInfo, err := c.cmdsInfoCache.Get()
	if err != nil {
		return nil
	}
	info := cmdsInfo[name]
	if info == nil {
		internal.Logf("info for cmd=%s not found", name)
	}
	return info
}

func (c *Ring) cmdShard(cmd Cmder) (*ringShard, error) {
	cmdInfo := c.cmdInfo(cmd.Name())
	pos := cmdFirstKeyPos(cmd, cmdInfo)
	if pos == 0 {
		return c.shards.Random()
	}
	firstKey := cmd.stringArg(pos)
	return c.shards.GetByKey(firstKey)
}

// Do creates a Cmd from the args and processes the cmd.
func (c *Ring) Do(args ...interface{}) *Cmd {
	cmd := NewCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *Ring) WrapProcess(
	fn func(oldProcess func(cmd Cmder) error) func(cmd Cmder) error,
) {
	c.process = fn(c.process)
}

func (c *Ring) Process(cmd Cmder) error {
	return c.process(cmd)
}

func (c *Ring) defaultProcess(cmd Cmder) error {
	for attempt := 0; attempt <= c.opt.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.retryBackoff(attempt))
		}

		shard, err := c.cmdShard(cmd)
		if err != nil {
			cmd.setErr(err)
			return err
		}

		err = shard.Client.Process(cmd)
		if err == nil {
			return nil
		}
		if !internal.IsRetryableError(err, cmd.readTimeout() == nil) {
			return err
		}
	}
	return cmd.Err()
}

func (c *Ring) Pipeline() Pipeliner {
	pipe := Pipeline{
		exec: c.processPipeline,
	}
	pipe.cmdable.setProcessor(pipe.Process)
	return &pipe
}

func (c *Ring) Pipelined(fn func(Pipeliner) error) ([]Cmder, error) {
	return c.Pipeline().Pipelined(fn)
}

func (c *Ring) WrapProcessPipeline(
	fn func(oldProcess func([]Cmder) error) func([]Cmder) error,
) {
	c.processPipeline = fn(c.processPipeline)
}

func (c *Ring) defaultProcessPipeline(cmds []Cmder) error {
	cmdsMap := make(map[string][]Cmder)
	for _, cmd := range cmds {
		cmdInfo := c.cmdInfo(cmd.Name())
		hash := cmd.stringArg(cmdFirstKeyPos(cmd, cmdInfo))
		if hash != "" {
			hash = c.shards.Hash(hashtag.Key(hash))
		}
		cmdsMap[hash] = append(cmdsMap[hash], cmd)
	}

	for attempt := 0; attempt <= c.opt.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.retryBackoff(attempt))
		}

		var mu sync.Mutex
		var failedCmdsMap map[string][]Cmder
		var wg sync.WaitGroup

		for hash, cmds := range cmdsMap {
			wg.Add(1)
			go func(hash string, cmds []Cmder) {
				defer wg.Done()

				shard, err := c.shards.GetByHash(hash)
				if err != nil {
					setCmdsErr(cmds, err)
					return
				}

				cn, err := shard.Client.getConn()
				if err != nil {
					setCmdsErr(cmds, err)
					return
				}

				canRetry, err := shard.Client.pipelineProcessCmds(cn, cmds)
				shard.Client.releaseConnStrict(cn, err)

				if canRetry && internal.IsRetryableError(err, true) {
					mu.Lock()
					if failedCmdsMap == nil {
						failedCmdsMap = make(map[string][]Cmder)
					}
					failedCmdsMap[hash] = cmds
					mu.Unlock()
				}
			}(hash, cmds)
		}

		wg.Wait()
		if len(failedCmdsMap) == 0 {
			break
		}
		cmdsMap = failedCmdsMap
	}

	return cmdsFirstErr(cmds)
}

func (c *Ring) TxPipeline() Pipeliner {
	panic("not implemented")
}

func (c *Ring) TxPipelined(fn func(Pipeliner) error) ([]Cmder, error) {
	panic("not implemented")
}

// Close closes the ring client, releasing any open resources.
//
// It is rare to Close a Ring, as the Ring is meant to be long-lived
// and shared between many goroutines.
func (c *Ring) Close() error {
	return c.shards.Close()
}

func (c *Ring) Watch(fn func(*Tx) error, keys ...string) error {
	if len(keys) == 0 {
		return fmt.Errorf("redis: Watch requires at least one key")
	}

	var shards []*ringShard
	for _, key := range keys {
		if key != "" {
			shard, err := c.shards.GetByKey(hashtag.Key(key))
			if err != nil {
				return err
			}

			shards = append(shards, shard)
		}
	}

	if len(shards) == 0 {
		return fmt.Errorf("redis: Watch requires at least one shard")
	}

	if len(shards) > 1 {
		for _, shard := range shards[1:] {
			if shard.Client != shards[0].Client {
				err := fmt.Errorf("redis: Watch requires all keys to be in the same shard")
				return err
			}
		}
	}

	return shards[0].Client.Watch(fn, keys...)
}

func newConsistentHash(opt *RingOptions) *consistenthash.Map {
	return consistenthash.New(opt.HashReplicas, consistenthash.Hash(opt.Hash))
}
