package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v7/internal"
	"github.com/go-redis/redis/v7/internal/pool"
	"github.com/go-redis/redis/v7/internal/proto"
)

// Nil reply returned by Redis when key does not exist.
const Nil = proto.Nil

func SetLogger(logger *log.Logger) {
	internal.Logger = logger
}

//------------------------------------------------------------------------------

type Hook interface {
	BeforeProcess(ctx context.Context, cmd Cmder) (context.Context, error)
	AfterProcess(ctx context.Context, cmd Cmder) error

	BeforeProcessPipeline(ctx context.Context, cmds []Cmder) (context.Context, error)
	AfterProcessPipeline(ctx context.Context, cmds []Cmder) error
}

type hooks struct {
	hooks []Hook
}

func (hs *hooks) lock() {
	hs.hooks = hs.hooks[:len(hs.hooks):len(hs.hooks)]
}

func (hs hooks) clone() hooks {
	clone := hs
	clone.lock()
	return clone
}

func (hs *hooks) AddHook(hook Hook) {
	hs.hooks = append(hs.hooks, hook)
}

func (hs hooks) process(
	ctx context.Context, cmd Cmder, fn func(context.Context, Cmder) error,
) error {
	ctx, err := hs.beforeProcess(ctx, cmd)
	if err != nil {
		cmd.SetErr(err)
		return err
	}

	cmdErr := fn(ctx, cmd)

	if err := hs.afterProcess(ctx, cmd); err != nil {
		cmd.SetErr(err)
		return err
	}

	return cmdErr
}

func (hs hooks) beforeProcess(ctx context.Context, cmd Cmder) (context.Context, error) {
	for _, h := range hs.hooks {
		var err error
		ctx, err = h.BeforeProcess(ctx, cmd)
		if err != nil {
			return nil, err
		}
	}
	return ctx, nil
}

func (hs hooks) afterProcess(ctx context.Context, cmd Cmder) error {
	var firstErr error
	for _, h := range hs.hooks {
		err := h.AfterProcess(ctx, cmd)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (hs hooks) processPipeline(
	ctx context.Context, cmds []Cmder, fn func(context.Context, []Cmder) error,
) error {
	ctx, err := hs.beforeProcessPipeline(ctx, cmds)
	if err != nil {
		setCmdsErr(cmds, err)
		return err
	}

	cmdsErr := fn(ctx, cmds)

	if err := hs.afterProcessPipeline(ctx, cmds); err != nil {
		setCmdsErr(cmds, err)
		return err
	}

	return cmdsErr
}

func (hs hooks) beforeProcessPipeline(ctx context.Context, cmds []Cmder) (context.Context, error) {
	for _, h := range hs.hooks {
		var err error
		ctx, err = h.BeforeProcessPipeline(ctx, cmds)
		if err != nil {
			return nil, err
		}
	}
	return ctx, nil
}

func (hs hooks) afterProcessPipeline(ctx context.Context, cmds []Cmder) error {
	var firstErr error
	for _, h := range hs.hooks {
		err := h.AfterProcessPipeline(ctx, cmds)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (hs hooks) processTxPipeline(
	ctx context.Context, cmds []Cmder, fn func(context.Context, []Cmder) error,
) error {
	cmds = wrapMultiExec(cmds)
	return hs.processPipeline(ctx, cmds, fn)
}

//------------------------------------------------------------------------------

type baseClient struct {
	opt      *Options
	connPool pool.Pooler

	onClose func() error // hook called when client is closed
}

func newBaseClient(opt *Options, connPool pool.Pooler) *baseClient {
	return &baseClient{
		opt:      opt,
		connPool: connPool,
	}
}

func (c *baseClient) clone() *baseClient {
	clone := *c
	return &clone
}

func (c *baseClient) withTimeout(timeout time.Duration) *baseClient {
	opt := c.opt.clone()
	opt.ReadTimeout = timeout
	opt.WriteTimeout = timeout

	clone := c.clone()
	clone.opt = opt

	return clone
}

func (c *baseClient) String() string {
	return fmt.Sprintf("Redis<%s db:%d>", c.getAddr(), c.opt.DB)
}

func (c *baseClient) newConn(ctx context.Context) (*pool.Conn, error) {
	cn, err := c.connPool.NewConn(ctx)
	if err != nil {
		return nil, err
	}

	err = c.initConn(ctx, cn)
	if err != nil {
		_ = c.connPool.CloseConn(cn)
		return nil, err
	}

	return cn, nil
}

func (c *baseClient) getConn(ctx context.Context) (*pool.Conn, error) {
	if c.opt.Limiter != nil {
		err := c.opt.Limiter.Allow()
		if err != nil {
			return nil, err
		}
	}

	cn, err := c._getConn(ctx)
	if err != nil {
		if c.opt.Limiter != nil {
			c.opt.Limiter.ReportResult(err)
		}
		return nil, err
	}
	return cn, nil
}

func (c *baseClient) _getConn(ctx context.Context) (*pool.Conn, error) {
	cn, err := c.connPool.Get(ctx)
	if err != nil {
		return nil, err
	}

	err = c.initConn(ctx, cn)
	if err != nil {
		c.connPool.Remove(cn, err)
		if err := internal.Unwrap(err); err != nil {
			return nil, err
		}
		return nil, err
	}

	return cn, nil
}

func (c *baseClient) initConn(ctx context.Context, cn *pool.Conn) error {
	if cn.Inited {
		return nil
	}
	cn.Inited = true

	if c.opt.Password == "" &&
		c.opt.DB == 0 &&
		!c.opt.readOnly &&
		c.opt.OnConnect == nil {
		return nil
	}

	connPool := pool.NewSingleConnPool(nil)
	connPool.SetConn(cn)
	conn := newConn(ctx, c.opt, connPool)

	_, err := conn.Pipelined(func(pipe Pipeliner) error {
		if c.opt.Password != "" {
			if c.opt.Username != "" {
				pipe.AuthACL(c.opt.Username, c.opt.Password)
			} else {
				pipe.Auth(c.opt.Password)
			}
		}

		if c.opt.DB > 0 {
			pipe.Select(c.opt.DB)
		}

		if c.opt.readOnly {
			pipe.ReadOnly()
		}

		return nil
	})
	if err != nil {
		return err
	}

	if c.opt.OnConnect != nil {
		return c.opt.OnConnect(conn)
	}
	return nil
}

func (c *baseClient) releaseConn(cn *pool.Conn, err error) {
	if c.opt.Limiter != nil {
		c.opt.Limiter.ReportResult(err)
	}

	if isBadConn(err, false) {
		c.connPool.Remove(cn, err)
	} else {
		c.connPool.Put(cn)
	}
}

func (c *baseClient) withConn(
	ctx context.Context, fn func(context.Context, *pool.Conn) error,
) error {
	cn, err := c.getConn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		c.releaseConn(cn, err)
	}()

	err = fn(ctx, cn)
	return err
}

func (c *baseClient) process(ctx context.Context, cmd Cmder) error {
	err := c._process(ctx, cmd)
	if err != nil {
		cmd.SetErr(err)
		return err
	}
	return nil
}

func (c *baseClient) _process(ctx context.Context, cmd Cmder) error {
	var lastErr error
	for attempt := 0; attempt <= c.opt.MaxRetries; attempt++ {
		if attempt > 0 {
			if err := internal.Sleep(ctx, c.retryBackoff(attempt)); err != nil {
				return err
			}
		}

		retryTimeout := true
		lastErr = c.withConn(ctx, func(ctx context.Context, cn *pool.Conn) error {
			err := cn.WithWriter(ctx, c.opt.WriteTimeout, func(wr *proto.Writer) error {
				return writeCmd(wr, cmd)
			})
			if err != nil {
				return err
			}

			err = cn.WithReader(ctx, c.cmdTimeout(cmd), cmd.readReply)
			if err != nil {
				retryTimeout = cmd.readTimeout() == nil
				return err
			}

			return nil
		})
		if lastErr == nil || !isRetryableError(lastErr, retryTimeout) {
			return lastErr
		}
	}
	return lastErr
}

func (c *baseClient) retryBackoff(attempt int) time.Duration {
	return internal.RetryBackoff(attempt, c.opt.MinRetryBackoff, c.opt.MaxRetryBackoff)
}

func (c *baseClient) cmdTimeout(cmd Cmder) time.Duration {
	if timeout := cmd.readTimeout(); timeout != nil {
		t := *timeout
		if t == 0 {
			return 0
		}
		return t + 10*time.Second
	}
	return c.opt.ReadTimeout
}

// Close closes the client, releasing any open resources.
//
// It is rare to Close a Client, as the Client is meant to be
// long-lived and shared between many goroutines.
func (c *baseClient) Close() error {
	var firstErr error
	if c.onClose != nil {
		if err := c.onClose(); err != nil {
			firstErr = err
		}
	}
	if err := c.connPool.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (c *baseClient) getAddr() string {
	return c.opt.Addr
}

func (c *baseClient) processPipeline(ctx context.Context, cmds []Cmder) error {
	return c.generalProcessPipeline(ctx, cmds, c.pipelineProcessCmds)
}

func (c *baseClient) processTxPipeline(ctx context.Context, cmds []Cmder) error {
	return c.generalProcessPipeline(ctx, cmds, c.txPipelineProcessCmds)
}

type pipelineProcessor func(context.Context, *pool.Conn, []Cmder) (bool, error)

func (c *baseClient) generalProcessPipeline(
	ctx context.Context, cmds []Cmder, p pipelineProcessor,
) error {
	err := c._generalProcessPipeline(ctx, cmds, p)
	if err != nil {
		setCmdsErr(cmds, err)
		return err
	}
	return cmdsFirstErr(cmds)
}

func (c *baseClient) _generalProcessPipeline(
	ctx context.Context, cmds []Cmder, p pipelineProcessor,
) error {
	var lastErr error
	for attempt := 0; attempt <= c.opt.MaxRetries; attempt++ {
		if attempt > 0 {
			if err := internal.Sleep(ctx, c.retryBackoff(attempt)); err != nil {
				return err
			}
		}

		var canRetry bool
		lastErr = c.withConn(ctx, func(ctx context.Context, cn *pool.Conn) error {
			var err error
			canRetry, err = p(ctx, cn, cmds)
			return err
		})
		if lastErr == nil || !canRetry || !isRetryableError(lastErr, true) {
			return lastErr
		}
	}
	return lastErr
}

func (c *baseClient) pipelineProcessCmds(
	ctx context.Context, cn *pool.Conn, cmds []Cmder,
) (bool, error) {
	err := cn.WithWriter(ctx, c.opt.WriteTimeout, func(wr *proto.Writer) error {
		return writeCmds(wr, cmds)
	})
	if err != nil {
		return true, err
	}

	err = cn.WithReader(ctx, c.opt.ReadTimeout, func(rd *proto.Reader) error {
		return pipelineReadCmds(rd, cmds)
	})
	return true, err
}

func pipelineReadCmds(rd *proto.Reader, cmds []Cmder) error {
	for _, cmd := range cmds {
		err := cmd.readReply(rd)
		if err != nil && !isRedisError(err) {
			return err
		}
	}
	return nil
}

func (c *baseClient) txPipelineProcessCmds(
	ctx context.Context, cn *pool.Conn, cmds []Cmder,
) (bool, error) {
	err := cn.WithWriter(ctx, c.opt.WriteTimeout, func(wr *proto.Writer) error {
		return writeCmds(wr, cmds)
	})
	if err != nil {
		return true, err
	}

	err = cn.WithReader(ctx, c.opt.ReadTimeout, func(rd *proto.Reader) error {
		statusCmd := cmds[0].(*StatusCmd)
		// Trim multi and exec.
		cmds = cmds[1 : len(cmds)-1]

		err := txPipelineReadQueued(rd, statusCmd, cmds)
		if err != nil {
			return err
		}

		return pipelineReadCmds(rd, cmds)
	})
	return false, err
}

func wrapMultiExec(cmds []Cmder) []Cmder {
	if len(cmds) == 0 {
		panic("not reached")
	}
	cmds = append(cmds, make([]Cmder, 2)...)
	copy(cmds[1:], cmds[:len(cmds)-2])
	cmds[0] = NewStatusCmd("multi")
	cmds[len(cmds)-1] = NewSliceCmd("exec")
	return cmds
}

func txPipelineReadQueued(rd *proto.Reader, statusCmd *StatusCmd, cmds []Cmder) error {
	// Parse queued replies.
	if err := statusCmd.readReply(rd); err != nil {
		return err
	}

	for range cmds {
		if err := statusCmd.readReply(rd); err != nil && !isRedisError(err) {
			return err
		}
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
		err := fmt.Errorf("redis: expected '*', but got line %q", line)
		return err
	}

	return nil
}

//------------------------------------------------------------------------------

// Client is a Redis client representing a pool of zero or more
// underlying connections. It's safe for concurrent use by multiple
// goroutines.
type Client struct {
	*baseClient
	cmdable
	hooks
	ctx context.Context
}

// NewClient returns a client to the Redis Server specified by Options.
func NewClient(opt *Options) *Client {
	opt.init()

	c := Client{
		baseClient: newBaseClient(opt, newConnPool(opt)),
		ctx:        context.Background(),
	}
	c.cmdable = c.Process

	return &c
}

func (c *Client) clone() *Client {
	clone := *c
	clone.cmdable = clone.Process
	clone.hooks.lock()
	return &clone
}

func (c *Client) WithTimeout(timeout time.Duration) *Client {
	clone := c.clone()
	clone.baseClient = c.baseClient.withTimeout(timeout)
	return clone
}

func (c *Client) Context() context.Context {
	return c.ctx
}

func (c *Client) WithContext(ctx context.Context) *Client {
	if ctx == nil {
		panic("nil context")
	}
	clone := c.clone()
	clone.ctx = ctx
	return clone
}

func (c *Client) Conn() *Conn {
	return newConn(c.ctx, c.opt, pool.NewSingleConnPool(c.connPool))
}

// Do creates a Cmd from the args and processes the cmd.
func (c *Client) Do(args ...interface{}) *Cmd {
	return c.DoContext(c.ctx, args...)
}

func (c *Client) DoContext(ctx context.Context, args ...interface{}) *Cmd {
	cmd := NewCmd(args...)
	_ = c.ProcessContext(ctx, cmd)
	return cmd
}

func (c *Client) Process(cmd Cmder) error {
	return c.ProcessContext(c.ctx, cmd)
}

func (c *Client) ProcessContext(ctx context.Context, cmd Cmder) error {
	return c.hooks.process(ctx, cmd, c.baseClient.process)
}

func (c *Client) processPipeline(ctx context.Context, cmds []Cmder) error {
	return c.hooks.processPipeline(ctx, cmds, c.baseClient.processPipeline)
}

func (c *Client) processTxPipeline(ctx context.Context, cmds []Cmder) error {
	return c.hooks.processTxPipeline(ctx, cmds, c.baseClient.processTxPipeline)
}

// Options returns read-only Options that were used to create the client.
func (c *Client) Options() *Options {
	return c.opt
}

type PoolStats pool.Stats

// PoolStats returns connection pool stats.
func (c *Client) PoolStats() *PoolStats {
	stats := c.connPool.Stats()
	return (*PoolStats)(stats)
}

func (c *Client) Pipelined(fn func(Pipeliner) error) ([]Cmder, error) {
	return c.Pipeline().Pipelined(fn)
}

func (c *Client) Pipeline() Pipeliner {
	pipe := Pipeline{
		ctx:  c.ctx,
		exec: c.processPipeline,
	}
	pipe.init()
	return &pipe
}

func (c *Client) TxPipelined(fn func(Pipeliner) error) ([]Cmder, error) {
	return c.TxPipeline().Pipelined(fn)
}

// TxPipeline acts like Pipeline, but wraps queued commands with MULTI/EXEC.
func (c *Client) TxPipeline() Pipeliner {
	pipe := Pipeline{
		ctx:  c.ctx,
		exec: c.processTxPipeline,
	}
	pipe.init()
	return &pipe
}

func (c *Client) pubSub() *PubSub {
	pubsub := &PubSub{
		opt: c.opt,

		newConn: func(channels []string) (*pool.Conn, error) {
			return c.newConn(context.TODO())
		},
		closeConn: c.connPool.CloseConn,
	}
	pubsub.init()
	return pubsub
}

// Subscribe subscribes the client to the specified channels.
// Channels can be omitted to create empty subscription.
// Note that this method does not wait on a response from Redis, so the
// subscription may not be active immediately. To force the connection to wait,
// you may call the Receive() method on the returned *PubSub like so:
//
//    sub := client.Subscribe(queryResp)
//    iface, err := sub.Receive()
//    if err != nil {
//        // handle error
//    }
//
//    // Should be *Subscription, but others are possible if other actions have been
//    // taken on sub since it was created.
//    switch iface.(type) {
//    case *Subscription:
//        // subscribe succeeded
//    case *Message:
//        // received first message
//    case *Pong:
//        // pong received
//    default:
//        // handle error
//    }
//
//    ch := sub.Channel()
func (c *Client) Subscribe(channels ...string) *PubSub {
	pubsub := c.pubSub()
	if len(channels) > 0 {
		_ = pubsub.Subscribe(channels...)
	}
	return pubsub
}

// PSubscribe subscribes the client to the given patterns.
// Patterns can be omitted to create empty subscription.
func (c *Client) PSubscribe(channels ...string) *PubSub {
	pubsub := c.pubSub()
	if len(channels) > 0 {
		_ = pubsub.PSubscribe(channels...)
	}
	return pubsub
}

//------------------------------------------------------------------------------

type conn struct {
	baseClient
	cmdable
	statefulCmdable
}

// Conn is like Client, but its pool contains single connection.
type Conn struct {
	*conn
	ctx context.Context
}

func newConn(ctx context.Context, opt *Options, connPool pool.Pooler) *Conn {
	c := Conn{
		conn: &conn{
			baseClient: baseClient{
				opt:      opt,
				connPool: connPool,
			},
		},
		ctx: ctx,
	}
	c.cmdable = c.Process
	c.statefulCmdable = c.Process
	return &c
}

func (c *Conn) Process(cmd Cmder) error {
	return c.ProcessContext(c.ctx, cmd)
}

func (c *Conn) ProcessContext(ctx context.Context, cmd Cmder) error {
	return c.baseClient.process(ctx, cmd)
}

func (c *Conn) Pipelined(fn func(Pipeliner) error) ([]Cmder, error) {
	return c.Pipeline().Pipelined(fn)
}

func (c *Conn) Pipeline() Pipeliner {
	pipe := Pipeline{
		ctx:  c.ctx,
		exec: c.processPipeline,
	}
	pipe.init()
	return &pipe
}

func (c *Conn) TxPipelined(fn func(Pipeliner) error) ([]Cmder, error) {
	return c.TxPipeline().Pipelined(fn)
}

// TxPipeline acts like Pipeline, but wraps queued commands with MULTI/EXEC.
func (c *Conn) TxPipeline() Pipeliner {
	pipe := Pipeline{
		ctx:  c.ctx,
		exec: c.processTxPipeline,
	}
	pipe.init()
	return &pipe
}
