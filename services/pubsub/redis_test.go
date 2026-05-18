// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/nosql"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRedisConn = "redis://127.0.0.1:6379/0"

// redisAvailable is set by TestMain after starting (or finding) a redis-server.
// Each Redis-backed test skips when false.
var redisAvailable bool

func waitRedisReady(conn string, dur time.Duration) bool {
	ctxTimed, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for t := time.Now(); ; time.Sleep(50 * time.Millisecond) {
		if nosql.GetManager().GetRedisClient(conn).Ping(ctxTimed).Err() == nil {
			return true
		}
		if time.Since(t) > dur {
			return false
		}
	}
}

func TestMain(m *testing.M) {
	var cmd *exec.Cmd
	if waitRedisReady(testRedisConn, 0) {
		redisAvailable = true
	} else if prog, err := exec.LookPath("redis-server"); err == nil {
		cmd = &exec.Cmd{
			Path:   prog,
			Args:   []string{prog, "--bind", "127.0.0.1", "--port", "6379"},
			Dir:    os.TempDir(),
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}
		if err := cmd.Start(); err == nil && waitRedisReady(testRedisConn, 5*time.Second) {
			redisAvailable = true
		}
	}
	code := m.Run()
	if cmd != nil {
		_ = cmd.Process.Signal(os.Interrupt)
		_ = cmd.Wait()
	}
	os.Exit(code)
}

func requireRedis(t *testing.T) {
	t.Helper()
	if !redisAvailable {
		if os.Getenv("CI") != "" {
			t.Fatal("redis-server required in CI")
		}
		t.Skip("redis-server not available")
	}
}

func TestRedisBroker_SubscribePublish(t *testing.T) {
	requireRedis(t)
	topic := t.Name()

	b, err := NewRedisBroker(testRedisConn)
	require.NoError(t, err)

	ch, cancel := b.Subscribe(topic)
	defer cancel()

	b.Publish(topic, []byte("hello"))

	select {
	case msg := <-ch:
		assert.Equal(t, []byte("hello"), msg)
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive published message")
	}
}

func TestRedisBroker_TopicIsolation(t *testing.T) {
	requireRedis(t)
	topicA := t.Name() + "-a"
	topicB := t.Name() + "-b"

	b, err := NewRedisBroker(testRedisConn)
	require.NoError(t, err)

	chA, cancelA := b.Subscribe(topicA)
	defer cancelA()
	chB, cancelB := b.Subscribe(topicB)
	defer cancelB()

	b.Publish(topicA, []byte("only-a"))

	select {
	case msg := <-chA:
		assert.Equal(t, []byte("only-a"), msg)
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive message on topic a")
	}
	select {
	case msg := <-chB:
		t.Fatalf("topic b unexpectedly got message: %s", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestRedisBroker_FanOutToLocalSubscribers(t *testing.T) {
	requireRedis(t)
	topic := t.Name()

	b, err := NewRedisBroker(testRedisConn)
	require.NoError(t, err)

	const n = 3
	channels := make([]<-chan []byte, n)
	cancels := make([]func(), n)
	for i := range n {
		channels[i], cancels[i] = b.Subscribe(topic)
	}
	defer func() {
		for _, c := range cancels {
			c()
		}
	}()

	b.Publish(topic, []byte("broadcast"))

	for i, ch := range channels {
		select {
		case msg := <-ch:
			assert.Equal(t, []byte("broadcast"), msg, "subscriber %d", i)
		case <-time.After(2 * time.Second):
			t.Fatalf("subscriber %d did not receive message", i)
		}
	}
}

func TestRedisBroker_CancelClosesChannelAndCleansTopic(t *testing.T) {
	requireRedis(t)
	topic := t.Name()

	b, err := NewRedisBroker(testRedisConn)
	require.NoError(t, err)

	ch, cancel := b.Subscribe(topic)
	cancel()

	_, ok := <-ch
	assert.False(t, ok, "channel must be closed after cancel")

	b.mu.RLock()
	_, present := b.topics[topic]
	b.mu.RUnlock()
	assert.False(t, present, "topic state must be removed after last subscriber cancels")
}

func TestRedisBroker_HasTopicSubscribersAlwaysTrue(t *testing.T) {
	requireRedis(t)

	b, err := NewRedisBroker(testRedisConn)
	require.NoError(t, err)

	// Conservative answer so publishers don't skip cross-process subscribers.
	assert.True(t, b.HasTopicSubscribers("anything"))
}
