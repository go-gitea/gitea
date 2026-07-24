// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"gitea.dev/modules/nosql"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRedisConn is the DB used for pubsub tests. Redis pub/sub channels are
// instance-global (not DB-scoped), so the index does not isolate topics; it
// mirrors modules/queue only for consistency.
const testRedisConn = "redis://127.0.0.1:6379/0"

// waitRedisReady polls until the redis at conn answers PING or dur elapses.
// Duplicated from modules/queue's base_redis_test.go (package-private there).
func waitRedisReady(conn string, dur time.Duration) (ready bool) {
	ctxTimed, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	for t := time.Now(); ; time.Sleep(50 * time.Millisecond) {
		ret := nosql.GetManager().GetRedisClient(conn).Ping(ctxTimed)
		if ret.Err() == nil {
			return true
		}
		if time.Since(t) > dur {
			return false
		}
	}
}

// redisServerCmd builds a redis-server launcher, or nil when the binary is
// absent. Duplicated from modules/queue's base_redis_test.go.
func redisServerCmd(t *testing.T) *exec.Cmd {
	redisServerProg, err := exec.LookPath("redis-server")
	if err != nil {
		return nil
	}
	c := &exec.Cmd{
		Path:   redisServerProg,
		Args:   []string{redisServerProg, "--bind", "127.0.0.1", "--port", "6379"},
		Dir:    t.TempDir(),
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	return c
}

// startTestRedis makes a redis reachable at testRedisConn for the test's
// duration, reusing a running server or launching redis-server (killed on
// cleanup). Absent both, it skips locally or fails in CI via
// test.AllowSkipExternalService. Mirrors modules/queue's TestBaseRedis setup.
func startTestRedis(t *testing.T) string {
	if !waitRedisReady(testRedisConn, 0) {
		redisServer := redisServerCmd(t)
		if redisServer == nil && test.AllowSkipExternalService() {
			t.Skip("redis-server command not found, skipped")
		}
		require.NotNil(t, redisServer)
		assert.NoError(t, redisServer.Start())
		t.Cleanup(func() {
			_ = redisServer.Process.Signal(os.Interrupt)
			_ = redisServer.Wait()
		})
		require.True(t, waitRedisReady(testRedisConn, 5*time.Second), "start redis-server")
	}
	return testRedisConn
}

func newTestRedisBroker(t *testing.T) *RedisBroker {
	t.Helper()
	b, err := NewRedisBroker(startTestRedis(t))
	require.NoError(t, err)
	return b
}

// TestRedisBroker runs the shared broker scenarios against a real Redis-backed
// RedisBroker. Delivery crosses a Redis round-trip (hence the wider timeout) and
// HasTopicSubscribers answers conservatively true, so subscriber tracking is not
// exact.
func TestRedisBroker(t *testing.T) {
	startTestRedis(t) // skip early if redis is unavailable
	testBrokerBasic(t, func(t *testing.T) Broker {
		return newTestRedisBroker(t)
	}, 2*time.Second, false)
}

// Backend-specific: RedisBroker tears down its per-topic Redis subscription and
// internal state once the last local subscriber cancels.
func TestRedisBroker_CancelCleansTopicState(t *testing.T) {
	b := newTestRedisBroker(t)
	ch, cancel := b.Subscribe("topic")
	cancel()

	_, ok := <-ch
	assert.False(t, ok, "channel must be closed after cancel")

	b.mu.RLock()
	_, present := b.topics["topic"]
	b.mu.RUnlock()
	assert.False(t, present, "topic state must be removed after last subscriber cancels")
}

// Backend-specific: two RedisBroker instances sharing one Redis simulate two
// Gitea processes - a publish on one must reach a subscriber on the other.
func TestRedisBroker_CrossBroker(t *testing.T) {
	conn := startTestRedis(t)

	publisher, err := NewRedisBroker(conn)
	require.NoError(t, err)
	subscriber, err := NewRedisBroker(conn)
	require.NoError(t, err)

	ch, cancel := subscriber.Subscribe("topic")
	defer cancel()

	publisher.Publish("topic", []byte("cross-process"))

	select {
	case msg := <-ch:
		assert.Equal(t, []byte("cross-process"), msg)
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber on second broker did not receive message")
	}
}
