package redis

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/internal"
	"github.com/go-redis/redis/internal/pool"
	"github.com/go-redis/redis/internal/proto"
)

// Nil reply Redis returns when key does not exist.
const Nil = proto.Nil

func init() {
	SetLogger(log.New(os.Stderr, "redis: ", log.LstdFlags|log.Lshortfile))
}

func SetLogger(logger *log.Logger) {
	internal.Logger = logger
}

type baseClient struct {
	opt      *Options
	connPool pool.Pooler
	limiter  Limiter

	process           func(Cmder) error
	processPipeline   func([]Cmder) error
	processTxPipeline func([]Cmder) error

	onClose func() error // hook called when client is closed
}

func (c *baseClient) init() {
	c.process = c.defaultProcess
	c.processPipeline = c.defaultProcessPipeline
	c.processTxPipeline = c.defaultProcessTxPipeline
}

func (c *baseClient) String() string {
	return fmt.Sprintf("Redis<%s db:%d>", c.getAddr(), c.opt.DB)
}

func (c *baseClient) newConn() (*pool.Conn, error) {
	cn, err := c.connPool.NewConn()
	if err != nil {
		return nil, err
	}

	if cn.InitedAt.IsZero() {
		if err := c.initConn(cn); err != nil {
			_ = c.connPool.CloseConn(cn)
			return nil, err
		}
	}

	return cn, nil
}

func (c *baseClient) getConn() (*pool.Conn, error) {
	if c.limiter != nil {
		err := c.limiter.Allow()
		if err != nil {
			return nil, err
		}
	}

	cn, err := c._getConn()
	if err != nil {
		if c.limiter != nil {
			c.limiter.ReportResult(err)
		}
		return nil, err
	}
	return cn, nil
}

func (c *baseClient) _getConn() (*pool.Conn, error) {
	cn, err := c.connPool.Get()
	if err != nil {
		return nil, err
	}

	if cn.InitedAt.IsZero() {
		err := c.initConn(cn)
		if err != nil {
			c.connPool.Remove(cn)
			return nil, err
		}
	}

	return cn, nil
}

func (c *baseClient) releaseConn(cn *pool.Conn, err error) {
	if c.limiter != nil {
		c.limiter.ReportResult(err)
	}

	if internal.IsBadConn(err, false) {
		c.connPool.Remove(cn)
	} else {
		c.connPool.Put(cn)
	}
}

func (c *baseClient) releaseConnStrict(cn *pool.Conn, err error) {
	if c.limiter != nil {
		c.limiter.ReportResult(err)
	}

	if err == nil || internal.IsRedisError(err) {
		c.connPool.Put(cn)
	} else {
		c.connPool.Remove(cn)
	}
}

func (c *baseClient) initConn(cn *pool.Conn) error {
	cn.InitedAt = time.Now()

	if c.opt.Password == "" &&
		c.opt.DB == 0 &&
		!c.opt.readOnly &&
		c.opt.OnConnect == nil {
		return nil
	}

	conn := newConn(c.opt, cn)
	_, err := conn.Pipelined(func(pipe Pipeliner) error {
		if c.opt.Password != "" {
			pipe.Auth(c.opt.Password)
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

// Do creates a Cmd from the args and processes the cmd.
func (c *baseClient) Do(args ...interface{}) *Cmd {
	cmd := NewCmd(args...)
	_ = c.Process(cmd)
	return cmd
}

// WrapProcess wraps function that processes Redis commands.
func (c *baseClient) WrapProcess(
	fn func(oldProcess func(cmd Cmder) error) func(cmd Cmder) error,
) {
	c.process = fn(c.process)
}

func (c *baseClient) Process(cmd Cmder) error {
	return c.process(cmd)
}

func (c *baseClient) defaultProcess(cmd Cmder) error {
	for attempt := 0; attempt <= c.opt.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.retryBackoff(attempt))
		}

		cn, err := c.getConn()
		if err != nil {
			cmd.setErr(err)
			if internal.IsRetryableError(err, true) {
				continue
			}
			return err
		}

		err = cn.WithWriter(c.opt.WriteTimeout, func(wr *proto.Writer) error {
			return writeCmd(wr, cmd)
		})
		if err != nil {
			c.releaseConn(cn, err)
			cmd.setErr(err)
			if internal.IsRetryableError(err, true) {
				continue
			}
			return err
		}

		err = cn.WithReader(c.cmdTimeout(cmd), func(rd *proto.Reader) error {
			return cmd.readReply(rd)
		})
		c.releaseConn(cn, err)
		if err != nil && internal.IsRetryableError(err, cmd.readTimeout() == nil) {
			continue
		}

		return err
	}

	return cmd.Err()
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
		if err := c.onClose(); err != nil && firstErr == nil {
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

func (c *baseClient) WrapProcessPipeline(
	fn func(oldProcess func([]Cmder) error) func([]Cmder) error,
) {
	c.processPipeline = fn(c.processPipeline)
	c.processTxPipeline = fn(c.processTxPipeline)
}

func (c *baseClient) defaultProcessPipeline(cmds []Cmder) error {
	return c.generalProcessPipeline(cmds, c.pipelineProcessCmds)
}

func (c *baseClient) defaultProcessTxPipeline(cmds []Cmder) error {
	return c.generalProcessPipeline(cmds, c.txPipelineProcessCmds)
}

type pipelineProcessor func(*pool.Conn, []Cmder) (bool, error)

func (c *baseClient) generalProcessPipeline(cmds []Cmder, p pipelineProcessor) error {
	for attempt := 0; attempt <= c.opt.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.retryBackoff(attempt))
		}

		cn, err := c.getConn()
		if err != nil {
			setCmdsErr(cmds, err)
			return err
		}

		canRetry, err := p(cn, cmds)
		c.releaseConnStrict(cn, err)

		if !canRetry || !internal.IsRetryableError(err, true) {
			break
		}
	}
	return cmdsFirstErr(cmds)
}

func (c *baseClient) pipelineProcessCmds(cn *pool.Conn, cmds []Cmder) (bool, error) {
	err := cn.WithWriter(c.opt.WriteTimeout, func(wr *proto.Writer) error {
		return writeCmd(wr, cmds...)
	})
	if err != nil {
		setCmdsErr(cmds, err)
		return true, err
	}

	err = cn.WithReader(c.opt.ReadTimeout, func(rd *proto.Reader) error {
		return pipelineReadCmds(rd, cmds)
	})
	return true, err
}

func pipelineReadCmds(rd *proto.Reader, cmds []Cmder) error {
	for _, cmd := range cmds {
		err := cmd.readReply(rd)
		if err != nil && !internal.IsRedisError(err) {
			return err
		}
	}
	return nil
}

func (c *baseClient) txPipelineProcessCmds(cn *pool.Conn, cmds []Cmder) (bool, error) {
	err := cn.WithWriter(c.opt.WriteTimeout, func(wr *proto.Writer) error {
		return txPipelineWriteMulti(wr, cmds)
	})
	if err != nil {
		setCmdsErr(cmds, err)
		return true, err
	}

	err = cn.WithReader(c.opt.ReadTimeout, func(rd *proto.Reader) error {
		err := txPipelineReadQueued(rd, cmds)
		if err != nil {
			setCmdsErr(cmds, err)
			return err
		}
		return pipelineReadCmds(rd, cmds)
	})
	return false, err
}

func txPipelineWriteMulti(wr *proto.Writer, cmds []Cmder) error {
	multiExec := make([]Cmder, 0, len(cmds)+2)
	multiExec = append(multiExec, NewStatusCmd("MULTI"))
	multiExec = append(multiExec, cmds...)
	multiExec = append(multiExec, NewSliceCmd("EXEC"))
	return writeCmd(wr, multiExec...)
}

func txPipelineReadQueued(rd *proto.Reader, cmds []Cmder) error {
	// Parse queued replies.
	var statusCmd StatusCmd
	err := statusCmd.readReply(rd)
	if err != nil {
		return err
	}

	for range cmds {
		err = statusCmd.readReply(rd)
		if err != nil && !internal.IsRedisError(err) {
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
	baseClient
	cmdable

	ctx context.Context
}

// NewClient returns a client to the Redis Server specified by Options.
func NewClient(opt *Options) *Client {
	opt.init()

	c := Client{
		baseClient: baseClient{
			opt:      opt,
			connPool: newConnPool(opt),
		},
	}
	c.baseClient.init()
	c.init()

	return &c
}

func (c *Client) init() {
	c.cmdable.setProcessor(c.Process)
}

func (c *Client) Context() context.Context {
	if c.ctx != nil {
		return c.ctx
	}
	return context.Background()
}

func (c *Client) WithContext(ctx context.Context) *Client {
	if ctx == nil {
		panic("nil context")
	}
	c2 := c.clone()
	c2.ctx = ctx
	return c2
}

func (c *Client) clone() *Client {
	cp := *c
	cp.init()
	return &cp
}

// Options returns read-only Options that were used to create the client.
func (c *Client) Options() *Options {
	return c.opt
}

func (c *Client) SetLimiter(l Limiter) *Client {
	c.limiter = l
	return c
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
		exec: c.processPipeline,
	}
	pipe.statefulCmdable.setProcessor(pipe.Process)
	return &pipe
}

func (c *Client) TxPipelined(fn func(Pipeliner) error) ([]Cmder, error) {
	return c.TxPipeline().Pipelined(fn)
}

// TxPipeline acts like Pipeline, but wraps queued commands with MULTI/EXEC.
func (c *Client) TxPipeline() Pipeliner {
	pipe := Pipeline{
		exec: c.processTxPipeline,
	}
	pipe.statefulCmdable.setProcessor(pipe.Process)
	return &pipe
}

func (c *Client) pubSub() *PubSub {
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

// Conn is like Client, but its pool contains single connection.
type Conn struct {
	baseClient
	statefulCmdable
}

func newConn(opt *Options, cn *pool.Conn) *Conn {
	c := Conn{
		baseClient: baseClient{
			opt:      opt,
			connPool: pool.NewSingleConnPool(cn),
		},
	}
	c.baseClient.init()
	c.statefulCmdable.setProcessor(c.Process)
	return &c
}

func (c *Conn) Pipelined(fn func(Pipeliner) error) ([]Cmder, error) {
	return c.Pipeline().Pipelined(fn)
}

func (c *Conn) Pipeline() Pipeliner {
	pipe := Pipeline{
		exec: c.processPipeline,
	}
	pipe.statefulCmdable.setProcessor(pipe.Process)
	return &pipe
}

func (c *Conn) TxPipelined(fn func(Pipeliner) error) ([]Cmder, error) {
	return c.TxPipeline().Pipelined(fn)
}

// TxPipeline acts like Pipeline, but wraps queued commands with MULTI/EXEC.
func (c *Conn) TxPipeline() Pipeliner {
	pipe := Pipeline{
		exec: c.processTxPipeline,
	}
	pipe.statefulCmdable.setProcessor(pipe.Process)
	return &pipe
}
