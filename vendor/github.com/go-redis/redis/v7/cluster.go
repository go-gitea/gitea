package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"math/rand"
	"net"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v7/internal"
	"github.com/go-redis/redis/v7/internal/hashtag"
	"github.com/go-redis/redis/v7/internal/pool"
	"github.com/go-redis/redis/v7/internal/proto"
)

var errClusterNoNodes = fmt.Errorf("redis: cluster has no nodes")

// ClusterOptions are used to configure a cluster client and should be
// passed to NewClusterClient.
type ClusterOptions struct {
	// A seed list of host:port addresses of cluster nodes.
	Addrs []string

	// The maximum number of retries before giving up. Command is retried
	// on network errors and MOVED/ASK redirects.
	// Default is 8 retries.
	MaxRedirects int

	// Enables read-only commands on slave nodes.
	ReadOnly bool
	// Allows routing read-only commands to the closest master or slave node.
	// It automatically enables ReadOnly.
	RouteByLatency bool
	// Allows routing read-only commands to the random master or slave node.
	// It automatically enables ReadOnly.
	RouteRandomly bool

	// Optional function that returns cluster slots information.
	// It is useful to manually create cluster of standalone Redis servers
	// and load-balance read/write operations between master and slaves.
	// It can use service like ZooKeeper to maintain configuration information
	// and Cluster.ReloadState to manually trigger state reloading.
	ClusterSlots func() ([]ClusterSlot, error)

	// Optional hook that is called when a new node is created.
	OnNewNode func(*Client)

	// Following options are copied from Options struct.

	Dialer func(ctx context.Context, network, addr string) (net.Conn, error)

	OnConnect func(*Conn) error

	Username string
	Password string

	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration

	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// NewClient creates a cluster node client with provided name and options.
	NewClient func(opt *Options) *Client

	// PoolSize applies per cluster node and not for the whole cluster.
	PoolSize           int
	MinIdleConns       int
	MaxConnAge         time.Duration
	PoolTimeout        time.Duration
	IdleTimeout        time.Duration
	IdleCheckFrequency time.Duration

	TLSConfig *tls.Config
}

func (opt *ClusterOptions) init() {
	if opt.MaxRedirects == -1 {
		opt.MaxRedirects = 0
	} else if opt.MaxRedirects == 0 {
		opt.MaxRedirects = 8
	}

	if (opt.RouteByLatency || opt.RouteRandomly) && opt.ClusterSlots == nil {
		opt.ReadOnly = true
	}

	if opt.PoolSize == 0 {
		opt.PoolSize = 5 * runtime.NumCPU()
	}

	switch opt.ReadTimeout {
	case -1:
		opt.ReadTimeout = 0
	case 0:
		opt.ReadTimeout = 3 * time.Second
	}
	switch opt.WriteTimeout {
	case -1:
		opt.WriteTimeout = 0
	case 0:
		opt.WriteTimeout = opt.ReadTimeout
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

	if opt.NewClient == nil {
		opt.NewClient = NewClient
	}
}

func (opt *ClusterOptions) clientOptions() *Options {
	const disableIdleCheck = -1

	return &Options{
		Dialer:    opt.Dialer,
		OnConnect: opt.OnConnect,

		MaxRetries:      opt.MaxRetries,
		MinRetryBackoff: opt.MinRetryBackoff,
		MaxRetryBackoff: opt.MaxRetryBackoff,
		Username:        opt.Username,
		Password:        opt.Password,
		readOnly:        opt.ReadOnly,

		DialTimeout:  opt.DialTimeout,
		ReadTimeout:  opt.ReadTimeout,
		WriteTimeout: opt.WriteTimeout,

		PoolSize:           opt.PoolSize,
		MinIdleConns:       opt.MinIdleConns,
		MaxConnAge:         opt.MaxConnAge,
		PoolTimeout:        opt.PoolTimeout,
		IdleTimeout:        opt.IdleTimeout,
		IdleCheckFrequency: disableIdleCheck,

		TLSConfig: opt.TLSConfig,
	}
}

//------------------------------------------------------------------------------

type clusterNode struct {
	Client *Client

	latency    uint32 // atomic
	generation uint32 // atomic
	failing    uint32 // atomic
}

func newClusterNode(clOpt *ClusterOptions, addr string) *clusterNode {
	opt := clOpt.clientOptions()
	opt.Addr = addr
	node := clusterNode{
		Client: clOpt.NewClient(opt),
	}

	node.latency = math.MaxUint32
	if clOpt.RouteByLatency {
		go node.updateLatency()
	}

	if clOpt.OnNewNode != nil {
		clOpt.OnNewNode(node.Client)
	}

	return &node
}

func (n *clusterNode) String() string {
	return n.Client.String()
}

func (n *clusterNode) Close() error {
	return n.Client.Close()
}

func (n *clusterNode) updateLatency() {
	const probes = 10

	var latency uint32
	for i := 0; i < probes; i++ {
		start := time.Now()
		n.Client.Ping()
		probe := uint32(time.Since(start) / time.Microsecond)
		latency = (latency + probe) / 2
	}
	atomic.StoreUint32(&n.latency, latency)
}

func (n *clusterNode) Latency() time.Duration {
	latency := atomic.LoadUint32(&n.latency)
	return time.Duration(latency) * time.Microsecond
}

func (n *clusterNode) MarkAsFailing() {
	atomic.StoreUint32(&n.failing, uint32(time.Now().Unix()))
}

func (n *clusterNode) Failing() bool {
	const timeout = 15 // 15 seconds

	failing := atomic.LoadUint32(&n.failing)
	if failing == 0 {
		return false
	}
	if time.Now().Unix()-int64(failing) < timeout {
		return true
	}
	atomic.StoreUint32(&n.failing, 0)
	return false
}

func (n *clusterNode) Generation() uint32 {
	return atomic.LoadUint32(&n.generation)
}

func (n *clusterNode) SetGeneration(gen uint32) {
	for {
		v := atomic.LoadUint32(&n.generation)
		if gen < v || atomic.CompareAndSwapUint32(&n.generation, v, gen) {
			break
		}
	}
}

//------------------------------------------------------------------------------

type clusterNodes struct {
	opt *ClusterOptions

	mu           sync.RWMutex
	allAddrs     []string
	allNodes     map[string]*clusterNode
	clusterAddrs []string
	closed       bool

	_generation uint32 // atomic
}

func newClusterNodes(opt *ClusterOptions) *clusterNodes {
	return &clusterNodes{
		opt: opt,

		allAddrs: opt.Addrs,
		allNodes: make(map[string]*clusterNode),
	}
}

func (c *clusterNodes) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	var firstErr error
	for _, node := range c.allNodes {
		if err := node.Client.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	c.allNodes = nil
	c.clusterAddrs = nil

	return firstErr
}

func (c *clusterNodes) Addrs() ([]string, error) {
	var addrs []string
	c.mu.RLock()
	closed := c.closed
	if !closed {
		if len(c.clusterAddrs) > 0 {
			addrs = c.clusterAddrs
		} else {
			addrs = c.allAddrs
		}
	}
	c.mu.RUnlock()

	if closed {
		return nil, pool.ErrClosed
	}
	if len(addrs) == 0 {
		return nil, errClusterNoNodes
	}
	return addrs, nil
}

func (c *clusterNodes) NextGeneration() uint32 {
	return atomic.AddUint32(&c._generation, 1)
}

// GC removes unused nodes.
func (c *clusterNodes) GC(generation uint32) {
	//nolint:prealloc
	var collected []*clusterNode
	c.mu.Lock()
	for addr, node := range c.allNodes {
		if node.Generation() >= generation {
			continue
		}

		c.clusterAddrs = remove(c.clusterAddrs, addr)
		delete(c.allNodes, addr)
		collected = append(collected, node)
	}
	c.mu.Unlock()

	for _, node := range collected {
		_ = node.Client.Close()
	}
}

func (c *clusterNodes) Get(addr string) (*clusterNode, error) {
	node, err := c.get(addr)
	if err != nil {
		return nil, err
	}
	if node != nil {
		return node, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, pool.ErrClosed
	}

	node, ok := c.allNodes[addr]
	if ok {
		return node, err
	}

	node = newClusterNode(c.opt, addr)

	c.allAddrs = appendIfNotExists(c.allAddrs, addr)
	c.clusterAddrs = append(c.clusterAddrs, addr)
	c.allNodes[addr] = node

	return node, err
}

func (c *clusterNodes) get(addr string) (*clusterNode, error) {
	var node *clusterNode
	var err error
	c.mu.RLock()
	if c.closed {
		err = pool.ErrClosed
	} else {
		node = c.allNodes[addr]
	}
	c.mu.RUnlock()
	return node, err
}

func (c *clusterNodes) All() ([]*clusterNode, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, pool.ErrClosed
	}

	cp := make([]*clusterNode, 0, len(c.allNodes))
	for _, node := range c.allNodes {
		cp = append(cp, node)
	}
	return cp, nil
}

func (c *clusterNodes) Random() (*clusterNode, error) {
	addrs, err := c.Addrs()
	if err != nil {
		return nil, err
	}

	n := rand.Intn(len(addrs))
	return c.Get(addrs[n])
}

//------------------------------------------------------------------------------

type clusterSlot struct {
	start, end int
	nodes      []*clusterNode
}

type clusterSlotSlice []*clusterSlot

func (p clusterSlotSlice) Len() int {
	return len(p)
}

func (p clusterSlotSlice) Less(i, j int) bool {
	return p[i].start < p[j].start
}

func (p clusterSlotSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

type clusterState struct {
	nodes   *clusterNodes
	Masters []*clusterNode
	Slaves  []*clusterNode

	slots []*clusterSlot

	generation uint32
	createdAt  time.Time
}

func newClusterState(
	nodes *clusterNodes, slots []ClusterSlot, origin string,
) (*clusterState, error) {
	c := clusterState{
		nodes: nodes,

		slots: make([]*clusterSlot, 0, len(slots)),

		generation: nodes.NextGeneration(),
		createdAt:  time.Now(),
	}

	originHost, _, _ := net.SplitHostPort(origin)
	isLoopbackOrigin := isLoopback(originHost)

	for _, slot := range slots {
		var nodes []*clusterNode
		for i, slotNode := range slot.Nodes {
			addr := slotNode.Addr
			if !isLoopbackOrigin {
				addr = replaceLoopbackHost(addr, originHost)
			}

			node, err := c.nodes.Get(addr)
			if err != nil {
				return nil, err
			}

			node.SetGeneration(c.generation)
			nodes = append(nodes, node)

			if i == 0 {
				c.Masters = appendUniqueNode(c.Masters, node)
			} else {
				c.Slaves = appendUniqueNode(c.Slaves, node)
			}
		}

		c.slots = append(c.slots, &clusterSlot{
			start: slot.Start,
			end:   slot.End,
			nodes: nodes,
		})
	}

	sort.Sort(clusterSlotSlice(c.slots))

	time.AfterFunc(time.Minute, func() {
		nodes.GC(c.generation)
	})

	return &c, nil
}

func replaceLoopbackHost(nodeAddr, originHost string) string {
	nodeHost, nodePort, err := net.SplitHostPort(nodeAddr)
	if err != nil {
		return nodeAddr
	}

	nodeIP := net.ParseIP(nodeHost)
	if nodeIP == nil {
		return nodeAddr
	}

	if !nodeIP.IsLoopback() {
		return nodeAddr
	}

	// Use origin host which is not loopback and node port.
	return net.JoinHostPort(originHost, nodePort)
}

func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return true
	}
	return ip.IsLoopback()
}

func (c *clusterState) slotMasterNode(slot int) (*clusterNode, error) {
	nodes := c.slotNodes(slot)
	if len(nodes) > 0 {
		return nodes[0], nil
	}
	return c.nodes.Random()
}

func (c *clusterState) slotSlaveNode(slot int) (*clusterNode, error) {
	nodes := c.slotNodes(slot)
	switch len(nodes) {
	case 0:
		return c.nodes.Random()
	case 1:
		return nodes[0], nil
	case 2:
		if slave := nodes[1]; !slave.Failing() {
			return slave, nil
		}
		return nodes[0], nil
	default:
		var slave *clusterNode
		for i := 0; i < 10; i++ {
			n := rand.Intn(len(nodes)-1) + 1
			slave = nodes[n]
			if !slave.Failing() {
				return slave, nil
			}
		}

		// All slaves are loading - use master.
		return nodes[0], nil
	}
}

func (c *clusterState) slotClosestNode(slot int) (*clusterNode, error) {
	const threshold = time.Millisecond

	nodes := c.slotNodes(slot)
	if len(nodes) == 0 {
		return c.nodes.Random()
	}

	var node *clusterNode
	for _, n := range nodes {
		if n.Failing() {
			continue
		}
		if node == nil || node.Latency()-n.Latency() > threshold {
			node = n
		}
	}
	return node, nil
}

func (c *clusterState) slotRandomNode(slot int) (*clusterNode, error) {
	nodes := c.slotNodes(slot)
	if len(nodes) == 0 {
		return c.nodes.Random()
	}
	n := rand.Intn(len(nodes))
	return nodes[n], nil
}

func (c *clusterState) slotNodes(slot int) []*clusterNode {
	i := sort.Search(len(c.slots), func(i int) bool {
		return c.slots[i].end >= slot
	})
	if i >= len(c.slots) {
		return nil
	}
	x := c.slots[i]
	if slot >= x.start && slot <= x.end {
		return x.nodes
	}
	return nil
}

//------------------------------------------------------------------------------

type clusterStateHolder struct {
	load func() (*clusterState, error)

	state     atomic.Value
	reloading uint32 // atomic
}

func newClusterStateHolder(fn func() (*clusterState, error)) *clusterStateHolder {
	return &clusterStateHolder{
		load: fn,
	}
}

func (c *clusterStateHolder) Reload() (*clusterState, error) {
	state, err := c.load()
	if err != nil {
		return nil, err
	}
	c.state.Store(state)
	return state, nil
}

func (c *clusterStateHolder) LazyReload() {
	if !atomic.CompareAndSwapUint32(&c.reloading, 0, 1) {
		return
	}
	go func() {
		defer atomic.StoreUint32(&c.reloading, 0)

		_, err := c.Reload()
		if err != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}()
}

func (c *clusterStateHolder) Get() (*clusterState, error) {
	v := c.state.Load()
	if v != nil {
		state := v.(*clusterState)
		if time.Since(state.createdAt) > time.Minute {
			c.LazyReload()
		}
		return state, nil
	}
	return c.Reload()
}

func (c *clusterStateHolder) ReloadOrGet() (*clusterState, error) {
	state, err := c.Reload()
	if err == nil {
		return state, nil
	}
	return c.Get()
}

//------------------------------------------------------------------------------

type clusterClient struct {
	opt           *ClusterOptions
	nodes         *clusterNodes
	state         *clusterStateHolder //nolint:structcheck
	cmdsInfoCache *cmdsInfoCache      //nolint:structcheck
}

// ClusterClient is a Redis Cluster client representing a pool of zero
// or more underlying connections. It's safe for concurrent use by
// multiple goroutines.
type ClusterClient struct {
	*clusterClient
	cmdable
	hooks
	ctx context.Context
}

// NewClusterClient returns a Redis Cluster client as described in
// http://redis.io/topics/cluster-spec.
func NewClusterClient(opt *ClusterOptions) *ClusterClient {
	opt.init()

	c := &ClusterClient{
		clusterClient: &clusterClient{
			opt:   opt,
			nodes: newClusterNodes(opt),
		},
		ctx: context.Background(),
	}
	c.state = newClusterStateHolder(c.loadState)
	c.cmdsInfoCache = newCmdsInfoCache(c.cmdsInfo)
	c.cmdable = c.Process

	if opt.IdleCheckFrequency > 0 {
		go c.reaper(opt.IdleCheckFrequency)
	}

	return c
}

func (c *ClusterClient) Context() context.Context {
	return c.ctx
}

func (c *ClusterClient) WithContext(ctx context.Context) *ClusterClient {
	if ctx == nil {
		panic("nil context")
	}
	clone := *c
	clone.cmdable = clone.Process
	clone.hooks.lock()
	clone.ctx = ctx
	return &clone
}

// Options returns read-only Options that were used to create the client.
func (c *ClusterClient) Options() *ClusterOptions {
	return c.opt
}

// ReloadState reloads cluster state. If available it calls ClusterSlots func
// to get cluster slots information.
func (c *ClusterClient) ReloadState() error {
	_, err := c.state.Reload()
	return err
}

// Close closes the cluster client, releasing any open resources.
//
// It is rare to Close a ClusterClient, as the ClusterClient is meant
// to be long-lived and shared between many goroutines.
func (c *ClusterClient) Close() error {
	return c.nodes.Close()
}

// Do creates a Cmd from the args and processes the cmd.
func (c *ClusterClient) Do(args ...interface{}) *Cmd {
	return c.DoContext(c.ctx, args...)
}

func (c *ClusterClient) DoContext(ctx context.Context, args ...interface{}) *Cmd {
	cmd := NewCmd(args...)
	_ = c.ProcessContext(ctx, cmd)
	return cmd
}

func (c *ClusterClient) Process(cmd Cmder) error {
	return c.ProcessContext(c.ctx, cmd)
}

func (c *ClusterClient) ProcessContext(ctx context.Context, cmd Cmder) error {
	return c.hooks.process(ctx, cmd, c.process)
}

func (c *ClusterClient) process(ctx context.Context, cmd Cmder) error {
	err := c._process(ctx, cmd)
	if err != nil {
		cmd.SetErr(err)
		return err
	}
	return nil
}

func (c *ClusterClient) _process(ctx context.Context, cmd Cmder) error {
	cmdInfo := c.cmdInfo(cmd.Name())
	slot := c.cmdSlot(cmd)

	var node *clusterNode
	var ask bool
	var lastErr error
	for attempt := 0; attempt <= c.opt.MaxRedirects; attempt++ {
		if attempt > 0 {
			if err := internal.Sleep(ctx, c.retryBackoff(attempt)); err != nil {
				return err
			}
		}

		if node == nil {
			var err error
			node, err = c.cmdNode(cmdInfo, slot)
			if err != nil {
				return err
			}
		}

		if ask {
			pipe := node.Client.Pipeline()
			_ = pipe.Process(NewCmd("asking"))
			_ = pipe.Process(cmd)
			_, lastErr = pipe.ExecContext(ctx)
			_ = pipe.Close()
			ask = false
		} else {
			lastErr = node.Client.ProcessContext(ctx, cmd)
		}

		// If there is no error - we are done.
		if lastErr == nil {
			return nil
		}
		if lastErr != Nil {
			c.state.LazyReload()
		}
		if lastErr == pool.ErrClosed || isReadOnlyError(lastErr) {
			node = nil
			continue
		}

		// If slave is loading - pick another node.
		if c.opt.ReadOnly && isLoadingError(lastErr) {
			node.MarkAsFailing()
			node = nil
			continue
		}

		var moved bool
		var addr string
		moved, ask, addr = isMovedError(lastErr)
		if moved || ask {
			var err error
			node, err = c.nodes.Get(addr)
			if err != nil {
				return err
			}
			continue
		}

		if isRetryableError(lastErr, cmd.readTimeout() == nil) {
			// First retry the same node.
			if attempt == 0 {
				continue
			}

			// Second try another node.
			node.MarkAsFailing()
			node = nil
			continue
		}

		return lastErr
	}
	return lastErr
}

// ForEachMaster concurrently calls the fn on each master node in the cluster.
// It returns the first error if any.
func (c *ClusterClient) ForEachMaster(fn func(client *Client) error) error {
	state, err := c.state.ReloadOrGet()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	for _, master := range state.Masters {
		wg.Add(1)
		go func(node *clusterNode) {
			defer wg.Done()
			err := fn(node.Client)
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
			}
		}(master)
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

// ForEachSlave concurrently calls the fn on each slave node in the cluster.
// It returns the first error if any.
func (c *ClusterClient) ForEachSlave(fn func(client *Client) error) error {
	state, err := c.state.ReloadOrGet()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	for _, slave := range state.Slaves {
		wg.Add(1)
		go func(node *clusterNode) {
			defer wg.Done()
			err := fn(node.Client)
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
			}
		}(slave)
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

// ForEachNode concurrently calls the fn on each known node in the cluster.
// It returns the first error if any.
func (c *ClusterClient) ForEachNode(fn func(client *Client) error) error {
	state, err := c.state.ReloadOrGet()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	worker := func(node *clusterNode) {
		defer wg.Done()
		err := fn(node.Client)
		if err != nil {
			select {
			case errCh <- err:
			default:
			}
		}
	}

	for _, node := range state.Masters {
		wg.Add(1)
		go worker(node)
	}
	for _, node := range state.Slaves {
		wg.Add(1)
		go worker(node)
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

// PoolStats returns accumulated connection pool stats.
func (c *ClusterClient) PoolStats() *PoolStats {
	var acc PoolStats

	state, _ := c.state.Get()
	if state == nil {
		return &acc
	}

	for _, node := range state.Masters {
		s := node.Client.connPool.Stats()
		acc.Hits += s.Hits
		acc.Misses += s.Misses
		acc.Timeouts += s.Timeouts

		acc.TotalConns += s.TotalConns
		acc.IdleConns += s.IdleConns
		acc.StaleConns += s.StaleConns
	}

	for _, node := range state.Slaves {
		s := node.Client.connPool.Stats()
		acc.Hits += s.Hits
		acc.Misses += s.Misses
		acc.Timeouts += s.Timeouts

		acc.TotalConns += s.TotalConns
		acc.IdleConns += s.IdleConns
		acc.StaleConns += s.StaleConns
	}

	return &acc
}

func (c *ClusterClient) loadState() (*clusterState, error) {
	if c.opt.ClusterSlots != nil {
		slots, err := c.opt.ClusterSlots()
		if err != nil {
			return nil, err
		}
		return newClusterState(c.nodes, slots, "")
	}

	addrs, err := c.nodes.Addrs()
	if err != nil {
		return nil, err
	}

	var firstErr error
	for _, addr := range addrs {
		node, err := c.nodes.Get(addr)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		slots, err := node.Client.ClusterSlots().Result()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		return newClusterState(c.nodes, slots, node.Client.opt.Addr)
	}

	return nil, firstErr
}

// reaper closes idle connections to the cluster.
func (c *ClusterClient) reaper(idleCheckFrequency time.Duration) {
	ticker := time.NewTicker(idleCheckFrequency)
	defer ticker.Stop()

	for range ticker.C {
		nodes, err := c.nodes.All()
		if err != nil {
			break
		}

		for _, node := range nodes {
			_, err := node.Client.connPool.(*pool.ConnPool).ReapStaleConns()
			if err != nil {
				internal.Logger.Printf("ReapStaleConns failed: %s", err)
			}
		}
	}
}

func (c *ClusterClient) Pipeline() Pipeliner {
	pipe := Pipeline{
		ctx:  c.ctx,
		exec: c.processPipeline,
	}
	pipe.init()
	return &pipe
}

func (c *ClusterClient) Pipelined(fn func(Pipeliner) error) ([]Cmder, error) {
	return c.Pipeline().Pipelined(fn)
}

func (c *ClusterClient) processPipeline(ctx context.Context, cmds []Cmder) error {
	return c.hooks.processPipeline(ctx, cmds, c._processPipeline)
}

func (c *ClusterClient) _processPipeline(ctx context.Context, cmds []Cmder) error {
	cmdsMap := newCmdsMap()
	err := c.mapCmdsByNode(cmdsMap, cmds)
	if err != nil {
		setCmdsErr(cmds, err)
		return err
	}

	for attempt := 0; attempt <= c.opt.MaxRedirects; attempt++ {
		if attempt > 0 {
			if err := internal.Sleep(ctx, c.retryBackoff(attempt)); err != nil {
				setCmdsErr(cmds, err)
				return err
			}
		}

		failedCmds := newCmdsMap()
		var wg sync.WaitGroup

		for node, cmds := range cmdsMap.m {
			wg.Add(1)
			go func(node *clusterNode, cmds []Cmder) {
				defer wg.Done()

				err := c._processPipelineNode(ctx, node, cmds, failedCmds)
				if err == nil {
					return
				}
				if attempt < c.opt.MaxRedirects {
					if err := c.mapCmdsByNode(failedCmds, cmds); err != nil {
						setCmdsErr(cmds, err)
					}
				} else {
					setCmdsErr(cmds, err)
				}
			}(node, cmds)
		}

		wg.Wait()
		if len(failedCmds.m) == 0 {
			break
		}
		cmdsMap = failedCmds
	}

	return cmdsFirstErr(cmds)
}

func (c *ClusterClient) mapCmdsByNode(cmdsMap *cmdsMap, cmds []Cmder) error {
	state, err := c.state.Get()
	if err != nil {
		return err
	}

	if c.opt.ReadOnly && c.cmdsAreReadOnly(cmds) {
		for _, cmd := range cmds {
			slot := c.cmdSlot(cmd)
			node, err := c.slotReadOnlyNode(state, slot)
			if err != nil {
				return err
			}
			cmdsMap.Add(node, cmd)
		}
		return nil
	}

	for _, cmd := range cmds {
		slot := c.cmdSlot(cmd)
		node, err := state.slotMasterNode(slot)
		if err != nil {
			return err
		}
		cmdsMap.Add(node, cmd)
	}
	return nil
}

func (c *ClusterClient) cmdsAreReadOnly(cmds []Cmder) bool {
	for _, cmd := range cmds {
		cmdInfo := c.cmdInfo(cmd.Name())
		if cmdInfo == nil || !cmdInfo.ReadOnly {
			return false
		}
	}
	return true
}

func (c *ClusterClient) _processPipelineNode(
	ctx context.Context, node *clusterNode, cmds []Cmder, failedCmds *cmdsMap,
) error {
	return node.Client.hooks.processPipeline(ctx, cmds, func(ctx context.Context, cmds []Cmder) error {
		return node.Client.withConn(ctx, func(ctx context.Context, cn *pool.Conn) error {
			err := cn.WithWriter(ctx, c.opt.WriteTimeout, func(wr *proto.Writer) error {
				return writeCmds(wr, cmds)
			})
			if err != nil {
				return err
			}

			return cn.WithReader(ctx, c.opt.ReadTimeout, func(rd *proto.Reader) error {
				return c.pipelineReadCmds(node, rd, cmds, failedCmds)
			})
		})
	})
}

func (c *ClusterClient) pipelineReadCmds(
	node *clusterNode, rd *proto.Reader, cmds []Cmder, failedCmds *cmdsMap,
) error {
	for _, cmd := range cmds {
		err := cmd.readReply(rd)
		if err == nil {
			continue
		}
		if c.checkMovedErr(cmd, err, failedCmds) {
			continue
		}

		if c.opt.ReadOnly && isLoadingError(err) {
			node.MarkAsFailing()
			return err
		}
		if isRedisError(err) {
			continue
		}
		return err
	}
	return nil
}

func (c *ClusterClient) checkMovedErr(
	cmd Cmder, err error, failedCmds *cmdsMap,
) bool {
	moved, ask, addr := isMovedError(err)
	if !moved && !ask {
		return false
	}

	node, err := c.nodes.Get(addr)
	if err != nil {
		return false
	}

	if moved {
		c.state.LazyReload()
		failedCmds.Add(node, cmd)
		return true
	}

	if ask {
		failedCmds.Add(node, NewCmd("asking"), cmd)
		return true
	}

	panic("not reached")
}

// TxPipeline acts like Pipeline, but wraps queued commands with MULTI/EXEC.
func (c *ClusterClient) TxPipeline() Pipeliner {
	pipe := Pipeline{
		ctx:  c.ctx,
		exec: c.processTxPipeline,
	}
	pipe.init()
	return &pipe
}

func (c *ClusterClient) TxPipelined(fn func(Pipeliner) error) ([]Cmder, error) {
	return c.TxPipeline().Pipelined(fn)
}

func (c *ClusterClient) processTxPipeline(ctx context.Context, cmds []Cmder) error {
	return c.hooks.processPipeline(ctx, cmds, c._processTxPipeline)
}

func (c *ClusterClient) _processTxPipeline(ctx context.Context, cmds []Cmder) error {
	state, err := c.state.Get()
	if err != nil {
		setCmdsErr(cmds, err)
		return err
	}

	cmdsMap := c.mapCmdsBySlot(cmds)
	for slot, cmds := range cmdsMap {
		node, err := state.slotMasterNode(slot)
		if err != nil {
			setCmdsErr(cmds, err)
			continue
		}

		cmdsMap := map[*clusterNode][]Cmder{node: cmds}
		for attempt := 0; attempt <= c.opt.MaxRedirects; attempt++ {
			if attempt > 0 {
				if err := internal.Sleep(ctx, c.retryBackoff(attempt)); err != nil {
					setCmdsErr(cmds, err)
					return err
				}
			}

			failedCmds := newCmdsMap()
			var wg sync.WaitGroup

			for node, cmds := range cmdsMap {
				wg.Add(1)
				go func(node *clusterNode, cmds []Cmder) {
					defer wg.Done()

					err := c._processTxPipelineNode(ctx, node, cmds, failedCmds)
					if err == nil {
						return
					}
					if attempt < c.opt.MaxRedirects {
						if err := c.mapCmdsByNode(failedCmds, cmds); err != nil {
							setCmdsErr(cmds, err)
						}
					} else {
						setCmdsErr(cmds, err)
					}
				}(node, cmds)
			}

			wg.Wait()
			if len(failedCmds.m) == 0 {
				break
			}
			cmdsMap = failedCmds.m
		}
	}

	return cmdsFirstErr(cmds)
}

func (c *ClusterClient) mapCmdsBySlot(cmds []Cmder) map[int][]Cmder {
	cmdsMap := make(map[int][]Cmder)
	for _, cmd := range cmds {
		slot := c.cmdSlot(cmd)
		cmdsMap[slot] = append(cmdsMap[slot], cmd)
	}
	return cmdsMap
}

func (c *ClusterClient) _processTxPipelineNode(
	ctx context.Context, node *clusterNode, cmds []Cmder, failedCmds *cmdsMap,
) error {
	return node.Client.hooks.processTxPipeline(ctx, cmds, func(ctx context.Context, cmds []Cmder) error {
		return node.Client.withConn(ctx, func(ctx context.Context, cn *pool.Conn) error {
			err := cn.WithWriter(ctx, c.opt.WriteTimeout, func(wr *proto.Writer) error {
				return writeCmds(wr, cmds)
			})
			if err != nil {
				return err
			}

			return cn.WithReader(ctx, c.opt.ReadTimeout, func(rd *proto.Reader) error {
				statusCmd := cmds[0].(*StatusCmd)
				// Trim multi and exec.
				cmds = cmds[1 : len(cmds)-1]

				err := c.txPipelineReadQueued(rd, statusCmd, cmds, failedCmds)
				if err != nil {
					moved, ask, addr := isMovedError(err)
					if moved || ask {
						return c.cmdsMoved(cmds, moved, ask, addr, failedCmds)
					}
					return err
				}

				return pipelineReadCmds(rd, cmds)
			})
		})
	})
}

func (c *ClusterClient) txPipelineReadQueued(
	rd *proto.Reader, statusCmd *StatusCmd, cmds []Cmder, failedCmds *cmdsMap,
) error {
	// Parse queued replies.
	if err := statusCmd.readReply(rd); err != nil {
		return err
	}

	for _, cmd := range cmds {
		err := statusCmd.readReply(rd)
		if err == nil || c.checkMovedErr(cmd, err, failedCmds) || isRedisError(err) {
			continue
		}
		return err
	}

	// Parse number of replies.
	line, err := rd.ReadLine()
	if err != nil {
		if err == Nil {
			err = TxFailedErr
		}
		return err
	}

	switch line[0] {
	case proto.ErrorReply:
		return proto.ParseErrorReply(line)
	case proto.ArrayReply:
		// ok
	default:
		return fmt.Errorf("redis: expected '*', but got line %q", line)
	}

	return nil
}

func (c *ClusterClient) cmdsMoved(
	cmds []Cmder, moved, ask bool, addr string, failedCmds *cmdsMap,
) error {
	node, err := c.nodes.Get(addr)
	if err != nil {
		return err
	}

	if moved {
		c.state.LazyReload()
		for _, cmd := range cmds {
			failedCmds.Add(node, cmd)
		}
		return nil
	}

	if ask {
		for _, cmd := range cmds {
			failedCmds.Add(node, NewCmd("asking"), cmd)
		}
		return nil
	}

	return nil
}

func (c *ClusterClient) Watch(fn func(*Tx) error, keys ...string) error {
	return c.WatchContext(c.ctx, fn, keys...)
}

func (c *ClusterClient) WatchContext(ctx context.Context, fn func(*Tx) error, keys ...string) error {
	if len(keys) == 0 {
		return fmt.Errorf("redis: Watch requires at least one key")
	}

	slot := hashtag.Slot(keys[0])
	for _, key := range keys[1:] {
		if hashtag.Slot(key) != slot {
			err := fmt.Errorf("redis: Watch requires all keys to be in the same slot")
			return err
		}
	}

	node, err := c.slotMasterNode(slot)
	if err != nil {
		return err
	}

	for attempt := 0; attempt <= c.opt.MaxRedirects; attempt++ {
		if attempt > 0 {
			if err := internal.Sleep(ctx, c.retryBackoff(attempt)); err != nil {
				return err
			}
		}

		err = node.Client.WatchContext(ctx, fn, keys...)
		if err == nil {
			break
		}
		if err != Nil {
			c.state.LazyReload()
		}

		moved, ask, addr := isMovedError(err)
		if moved || ask {
			node, err = c.nodes.Get(addr)
			if err != nil {
				return err
			}
			continue
		}

		if err == pool.ErrClosed || isReadOnlyError(err) {
			node, err = c.slotMasterNode(slot)
			if err != nil {
				return err
			}
			continue
		}

		if isRetryableError(err, true) {
			continue
		}

		return err
	}

	return err
}

func (c *ClusterClient) pubSub() *PubSub {
	var node *clusterNode
	pubsub := &PubSub{
		opt: c.opt.clientOptions(),

		newConn: func(channels []string) (*pool.Conn, error) {
			if node != nil {
				panic("node != nil")
			}

			var err error
			if len(channels) > 0 {
				slot := hashtag.Slot(channels[0])
				node, err = c.slotMasterNode(slot)
			} else {
				node, err = c.nodes.Random()
			}
			if err != nil {
				return nil, err
			}

			cn, err := node.Client.newConn(context.TODO())
			if err != nil {
				node = nil

				return nil, err
			}

			return cn, nil
		},
		closeConn: func(cn *pool.Conn) error {
			err := node.Client.connPool.CloseConn(cn)
			node = nil
			return err
		},
	}
	pubsub.init()

	return pubsub
}

// Subscribe subscribes the client to the specified channels.
// Channels can be omitted to create empty subscription.
func (c *ClusterClient) Subscribe(channels ...string) *PubSub {
	pubsub := c.pubSub()
	if len(channels) > 0 {
		_ = pubsub.Subscribe(channels...)
	}
	return pubsub
}

// PSubscribe subscribes the client to the given patterns.
// Patterns can be omitted to create empty subscription.
func (c *ClusterClient) PSubscribe(channels ...string) *PubSub {
	pubsub := c.pubSub()
	if len(channels) > 0 {
		_ = pubsub.PSubscribe(channels...)
	}
	return pubsub
}

func (c *ClusterClient) retryBackoff(attempt int) time.Duration {
	return internal.RetryBackoff(attempt, c.opt.MinRetryBackoff, c.opt.MaxRetryBackoff)
}

func (c *ClusterClient) cmdsInfo() (map[string]*CommandInfo, error) {
	addrs, err := c.nodes.Addrs()
	if err != nil {
		return nil, err
	}

	var firstErr error
	for _, addr := range addrs {
		node, err := c.nodes.Get(addr)
		if err != nil {
			return nil, err
		}
		if node == nil {
			continue
		}

		info, err := node.Client.Command().Result()
		if err == nil {
			return info, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, firstErr
}

func (c *ClusterClient) cmdInfo(name string) *CommandInfo {
	cmdsInfo, err := c.cmdsInfoCache.Get()
	if err != nil {
		return nil
	}

	info := cmdsInfo[name]
	if info == nil {
		internal.Logger.Printf("info for cmd=%s not found", name)
	}
	return info
}

func (c *ClusterClient) cmdSlot(cmd Cmder) int {
	args := cmd.Args()
	if args[0] == "cluster" && args[1] == "getkeysinslot" {
		return args[2].(int)
	}

	cmdInfo := c.cmdInfo(cmd.Name())
	return cmdSlot(cmd, cmdFirstKeyPos(cmd, cmdInfo))
}

func cmdSlot(cmd Cmder, pos int) int {
	if pos == 0 {
		return hashtag.RandomSlot()
	}
	firstKey := cmd.stringArg(pos)
	return hashtag.Slot(firstKey)
}

func (c *ClusterClient) cmdNode(cmdInfo *CommandInfo, slot int) (*clusterNode, error) {
	state, err := c.state.Get()
	if err != nil {
		return nil, err
	}

	if c.opt.ReadOnly && cmdInfo != nil && cmdInfo.ReadOnly {
		return c.slotReadOnlyNode(state, slot)
	}
	return state.slotMasterNode(slot)
}

func (c *clusterClient) slotReadOnlyNode(state *clusterState, slot int) (*clusterNode, error) {
	if c.opt.RouteByLatency {
		return state.slotClosestNode(slot)
	}
	if c.opt.RouteRandomly {
		return state.slotRandomNode(slot)
	}
	return state.slotSlaveNode(slot)
}

func (c *ClusterClient) slotMasterNode(slot int) (*clusterNode, error) {
	state, err := c.state.Get()
	if err != nil {
		return nil, err
	}
	return state.slotMasterNode(slot)
}

func appendUniqueNode(nodes []*clusterNode, node *clusterNode) []*clusterNode {
	for _, n := range nodes {
		if n == node {
			return nodes
		}
	}
	return append(nodes, node)
}

func appendIfNotExists(ss []string, es ...string) []string {
loop:
	for _, e := range es {
		for _, s := range ss {
			if s == e {
				continue loop
			}
		}
		ss = append(ss, e)
	}
	return ss
}

func remove(ss []string, es ...string) []string {
	if len(es) == 0 {
		return ss[:0]
	}
	for _, e := range es {
		for i, s := range ss {
			if s == e {
				ss = append(ss[:i], ss[i+1:]...)
				break
			}
		}
	}
	return ss
}

//------------------------------------------------------------------------------

type cmdsMap struct {
	mu sync.Mutex
	m  map[*clusterNode][]Cmder
}

func newCmdsMap() *cmdsMap {
	return &cmdsMap{
		m: make(map[*clusterNode][]Cmder),
	}
}

func (m *cmdsMap) Add(node *clusterNode, cmds ...Cmder) {
	m.mu.Lock()
	m.m[node] = append(m.m[node], cmds...)
	m.mu.Unlock()
}
