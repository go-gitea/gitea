package redis

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/internal"
	"github.com/go-redis/redis/internal/pool"
	"github.com/go-redis/redis/internal/proto"
)

var errPingTimeout = errors.New("redis: ping timeout")

// PubSub implements Pub/Sub commands bas described in
// http://redis.io/topics/pubsub. Message receiving is NOT safe
// for concurrent use by multiple goroutines.
//
// PubSub automatically reconnects to Redis Server and resubscribes
// to the channels in case of network errors.
type PubSub struct {
	opt *Options

	newConn   func([]string) (*pool.Conn, error)
	closeConn func(*pool.Conn) error

	mu       sync.Mutex
	cn       *pool.Conn
	channels map[string]struct{}
	patterns map[string]struct{}
	closed   bool
	exit     chan struct{}

	cmd *Cmd

	chOnce sync.Once
	ch     chan *Message
	ping   chan struct{}
}

func (c *PubSub) init() {
	c.exit = make(chan struct{})
}

func (c *PubSub) conn() (*pool.Conn, error) {
	c.mu.Lock()
	cn, err := c._conn(nil)
	c.mu.Unlock()
	return cn, err
}

func (c *PubSub) _conn(newChannels []string) (*pool.Conn, error) {
	if c.closed {
		return nil, pool.ErrClosed
	}
	if c.cn != nil {
		return c.cn, nil
	}

	channels := mapKeys(c.channels)
	channels = append(channels, newChannels...)

	cn, err := c.newConn(channels)
	if err != nil {
		return nil, err
	}

	if err := c.resubscribe(cn); err != nil {
		_ = c.closeConn(cn)
		return nil, err
	}

	c.cn = cn
	return cn, nil
}

func (c *PubSub) writeCmd(cn *pool.Conn, cmd Cmder) error {
	return cn.WithWriter(c.opt.WriteTimeout, func(wr *proto.Writer) error {
		return writeCmd(wr, cmd)
	})
}

func (c *PubSub) resubscribe(cn *pool.Conn) error {
	var firstErr error

	if len(c.channels) > 0 {
		err := c._subscribe(cn, "subscribe", mapKeys(c.channels))
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if len(c.patterns) > 0 {
		err := c._subscribe(cn, "psubscribe", mapKeys(c.patterns))
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func mapKeys(m map[string]struct{}) []string {
	s := make([]string, len(m))
	i := 0
	for k := range m {
		s[i] = k
		i++
	}
	return s
}

func (c *PubSub) _subscribe(
	cn *pool.Conn, redisCmd string, channels []string,
) error {
	args := make([]interface{}, 0, 1+len(channels))
	args = append(args, redisCmd)
	for _, channel := range channels {
		args = append(args, channel)
	}
	cmd := NewSliceCmd(args...)
	return c.writeCmd(cn, cmd)
}

func (c *PubSub) releaseConn(cn *pool.Conn, err error, allowTimeout bool) {
	c.mu.Lock()
	c._releaseConn(cn, err, allowTimeout)
	c.mu.Unlock()
}

func (c *PubSub) _releaseConn(cn *pool.Conn, err error, allowTimeout bool) {
	if c.cn != cn {
		return
	}
	if internal.IsBadConn(err, allowTimeout) {
		c._reconnect(err)
	}
}

func (c *PubSub) _reconnect(reason error) {
	_ = c._closeTheCn(reason)
	_, _ = c._conn(nil)
}

func (c *PubSub) _closeTheCn(reason error) error {
	if c.cn == nil {
		return nil
	}
	if !c.closed {
		internal.Logf("redis: discarding bad PubSub connection: %s", reason)
	}
	err := c.closeConn(c.cn)
	c.cn = nil
	return err
}

func (c *PubSub) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return pool.ErrClosed
	}
	c.closed = true
	close(c.exit)

	err := c._closeTheCn(pool.ErrClosed)
	return err
}

// Subscribe the client to the specified channels. It returns
// empty subscription if there are no channels.
func (c *PubSub) Subscribe(channels ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.subscribe("subscribe", channels...)
	if c.channels == nil {
		c.channels = make(map[string]struct{})
	}
	for _, s := range channels {
		c.channels[s] = struct{}{}
	}
	return err
}

// PSubscribe the client to the given patterns. It returns
// empty subscription if there are no patterns.
func (c *PubSub) PSubscribe(patterns ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.subscribe("psubscribe", patterns...)
	if c.patterns == nil {
		c.patterns = make(map[string]struct{})
	}
	for _, s := range patterns {
		c.patterns[s] = struct{}{}
	}
	return err
}

// Unsubscribe the client from the given channels, or from all of
// them if none is given.
func (c *PubSub) Unsubscribe(channels ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, channel := range channels {
		delete(c.channels, channel)
	}
	err := c.subscribe("unsubscribe", channels...)
	return err
}

// PUnsubscribe the client from the given patterns, or from all of
// them if none is given.
func (c *PubSub) PUnsubscribe(patterns ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, pattern := range patterns {
		delete(c.patterns, pattern)
	}
	err := c.subscribe("punsubscribe", patterns...)
	return err
}

func (c *PubSub) subscribe(redisCmd string, channels ...string) error {
	cn, err := c._conn(channels)
	if err != nil {
		return err
	}

	err = c._subscribe(cn, redisCmd, channels)
	c._releaseConn(cn, err, false)
	return err
}

func (c *PubSub) Ping(payload ...string) error {
	args := []interface{}{"ping"}
	if len(payload) == 1 {
		args = append(args, payload[0])
	}
	cmd := NewCmd(args...)

	cn, err := c.conn()
	if err != nil {
		return err
	}

	err = c.writeCmd(cn, cmd)
	c.releaseConn(cn, err, false)
	return err
}

// Subscription received after a successful subscription to channel.
type Subscription struct {
	// Can be "subscribe", "unsubscribe", "psubscribe" or "punsubscribe".
	Kind string
	// Channel name we have subscribed to.
	Channel string
	// Number of channels we are currently subscribed to.
	Count int
}

func (m *Subscription) String() string {
	return fmt.Sprintf("%s: %s", m.Kind, m.Channel)
}

// Message received as result of a PUBLISH command issued by another client.
type Message struct {
	Channel string
	Pattern string
	Payload string
}

func (m *Message) String() string {
	return fmt.Sprintf("Message<%s: %s>", m.Channel, m.Payload)
}

// Pong received as result of a PING command issued by another client.
type Pong struct {
	Payload string
}

func (p *Pong) String() string {
	if p.Payload != "" {
		return fmt.Sprintf("Pong<%s>", p.Payload)
	}
	return "Pong"
}

func (c *PubSub) newMessage(reply interface{}) (interface{}, error) {
	switch reply := reply.(type) {
	case string:
		return &Pong{
			Payload: reply,
		}, nil
	case []interface{}:
		switch kind := reply[0].(string); kind {
		case "subscribe", "unsubscribe", "psubscribe", "punsubscribe":
			return &Subscription{
				Kind:    kind,
				Channel: reply[1].(string),
				Count:   int(reply[2].(int64)),
			}, nil
		case "message":
			return &Message{
				Channel: reply[1].(string),
				Payload: reply[2].(string),
			}, nil
		case "pmessage":
			return &Message{
				Pattern: reply[1].(string),
				Channel: reply[2].(string),
				Payload: reply[3].(string),
			}, nil
		case "pong":
			return &Pong{
				Payload: reply[1].(string),
			}, nil
		default:
			return nil, fmt.Errorf("redis: unsupported pubsub message: %q", kind)
		}
	default:
		return nil, fmt.Errorf("redis: unsupported pubsub message: %#v", reply)
	}
}

// ReceiveTimeout acts like Receive but returns an error if message
// is not received in time. This is low-level API and in most cases
// Channel should be used instead.
func (c *PubSub) ReceiveTimeout(timeout time.Duration) (interface{}, error) {
	if c.cmd == nil {
		c.cmd = NewCmd()
	}

	cn, err := c.conn()
	if err != nil {
		return nil, err
	}

	err = cn.WithReader(timeout, func(rd *proto.Reader) error {
		return c.cmd.readReply(rd)
	})

	c.releaseConn(cn, err, timeout > 0)
	if err != nil {
		return nil, err
	}

	return c.newMessage(c.cmd.Val())
}

// Receive returns a message as a Subscription, Message, Pong or error.
// See PubSub example for details. This is low-level API and in most cases
// Channel should be used instead.
func (c *PubSub) Receive() (interface{}, error) {
	return c.ReceiveTimeout(0)
}

// ReceiveMessage returns a Message or error ignoring Subscription and Pong
// messages. This is low-level API and in most cases Channel should be used
// instead.
func (c *PubSub) ReceiveMessage() (*Message, error) {
	for {
		msg, err := c.Receive()
		if err != nil {
			return nil, err
		}

		switch msg := msg.(type) {
		case *Subscription:
			// Ignore.
		case *Pong:
			// Ignore.
		case *Message:
			return msg, nil
		default:
			err := fmt.Errorf("redis: unknown message: %T", msg)
			return nil, err
		}
	}
}

// Channel returns a Go channel for concurrently receiving messages.
// It periodically sends Ping messages to test connection health.
// The channel is closed with PubSub. Receive* APIs can not be used
// after channel is created.
func (c *PubSub) Channel() <-chan *Message {
	c.chOnce.Do(c.initChannel)
	return c.ch
}

func (c *PubSub) initChannel() {
	c.ch = make(chan *Message, 100)
	c.ping = make(chan struct{}, 10)

	go func() {
		var errCount int
		for {
			msg, err := c.Receive()
			if err != nil {
				if err == pool.ErrClosed {
					close(c.ch)
					return
				}
				if errCount > 0 {
					time.Sleep(c.retryBackoff(errCount))
				}
				errCount++
				continue
			}
			errCount = 0

			// Any message is as good as a ping.
			select {
			case c.ping <- struct{}{}:
			default:
			}

			switch msg := msg.(type) {
			case *Subscription:
				// Ignore.
			case *Pong:
				// Ignore.
			case *Message:
				c.ch <- msg
			default:
				internal.Logf("redis: unknown message: %T", msg)
			}
		}
	}()

	go func() {
		const timeout = 5 * time.Second

		timer := time.NewTimer(timeout)
		timer.Stop()

		healthy := true
		for {
			timer.Reset(timeout)
			select {
			case <-c.ping:
				healthy = true
				if !timer.Stop() {
					<-timer.C
				}
			case <-timer.C:
				pingErr := c.Ping()
				if healthy {
					healthy = false
				} else {
					if pingErr == nil {
						pingErr = errPingTimeout
					}
					c.mu.Lock()
					c._reconnect(pingErr)
					c.mu.Unlock()
				}
			case <-c.exit:
				return
			}
		}
	}()
}

func (c *PubSub) retryBackoff(attempt int) time.Duration {
	return internal.RetryBackoff(attempt, c.opt.MinRetryBackoff, c.opt.MaxRetryBackoff)
}
