package redis

import (
	"github.com/go-redis/redis/internal/pool"
	"github.com/go-redis/redis/internal/proto"
)

// TxFailedErr transaction redis failed.
const TxFailedErr = proto.RedisError("redis: transaction failed")

// Tx implements Redis transactions as described in
// http://redis.io/topics/transactions. It's NOT safe for concurrent use
// by multiple goroutines, because Exec resets list of watched keys.
// If you don't need WATCH it is better to use Pipeline.
type Tx struct {
	statefulCmdable
	baseClient
}

func (c *Client) newTx() *Tx {
	tx := Tx{
		baseClient: baseClient{
			opt:      c.opt,
			connPool: pool.NewStickyConnPool(c.connPool.(*pool.ConnPool), true),
		},
	}
	tx.baseClient.init()
	tx.statefulCmdable.setProcessor(tx.Process)
	return &tx
}

// Watch prepares a transaction and marks the keys to be watched
// for conditional execution if there are any keys.
//
// The transaction is automatically closed when fn exits.
func (c *Client) Watch(fn func(*Tx) error, keys ...string) error {
	tx := c.newTx()
	if len(keys) > 0 {
		if err := tx.Watch(keys...).Err(); err != nil {
			_ = tx.Close()
			return err
		}
	}

	err := fn(tx)
	_ = tx.Close()
	return err
}

// Close closes the transaction, releasing any open resources.
func (c *Tx) Close() error {
	_ = c.Unwatch().Err()
	return c.baseClient.Close()
}

// Watch marks the keys to be watched for conditional execution
// of a transaction.
func (c *Tx) Watch(keys ...string) *StatusCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "watch"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewStatusCmd(args...)
	c.Process(cmd)
	return cmd
}

// Unwatch flushes all the previously watched keys for a transaction.
func (c *Tx) Unwatch(keys ...string) *StatusCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "unwatch"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewStatusCmd(args...)
	c.Process(cmd)
	return cmd
}

// Pipeline creates a new pipeline. It is more convenient to use Pipelined.
func (c *Tx) Pipeline() Pipeliner {
	pipe := Pipeline{
		exec: c.processTxPipeline,
	}
	pipe.statefulCmdable.setProcessor(pipe.Process)
	return &pipe
}

// Pipelined executes commands queued in the fn in a transaction.
//
// When using WATCH, EXEC will execute commands only if the watched keys
// were not modified, allowing for a check-and-set mechanism.
//
// Exec always returns list of commands. If transaction fails
// TxFailedErr is returned. Otherwise Exec returns an error of the first
// failed command or nil.
func (c *Tx) Pipelined(fn func(Pipeliner) error) ([]Cmder, error) {
	return c.Pipeline().Pipelined(fn)
}

// TxPipelined is an alias for Pipelined.
func (c *Tx) TxPipelined(fn func(Pipeliner) error) ([]Cmder, error) {
	return c.Pipelined(fn)
}

// TxPipeline is an alias for Pipeline.
func (c *Tx) TxPipeline() Pipeliner {
	return c.Pipeline()
}
