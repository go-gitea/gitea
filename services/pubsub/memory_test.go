// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPubsub(t *testing.T) {
	var (
		wg          sync.WaitGroup
		testTopic   = "hello-world"
		testMessage = []byte("test")
	)

	ctx, cancel := context.WithCancelCause(
		context.Background(),
	)

	broker := NewMemory()
	go func() {
		broker.Subscribe(ctx, testTopic, func(message []byte) { assert.Equal(t, testMessage, message); wg.Done() })
	}()
	go func() {
		broker.Subscribe(ctx, testTopic, func(_ []byte) { wg.Done() })
	}()

	// Wait a bit for the subscriptions to be registered
	<-time.After(100 * time.Millisecond)

	wg.Add(2)
	go func() {
		broker.Publish(ctx, testTopic, testMessage)
	}()

	wg.Wait()
	cancel(nil)
}
