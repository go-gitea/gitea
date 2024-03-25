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
		wg sync.WaitGroup

		testMessage = Message{
			Data:  []byte("test"),
			Topic: "hello-world",
		}
	)

	ctx, cancel := context.WithCancelCause(
		context.Background(),
	)

	broker := NewMemory()
	go func() {
		broker.Subscribe(ctx, "hello-world", func(message Message) { assert.Equal(t, testMessage, message); wg.Done() })
	}()
	go func() {
		broker.Subscribe(ctx, "hello-world", func(_ Message) { wg.Done() })
	}()

	// Wait a bit for the subscriptions to be registered
	<-time.After(100 * time.Millisecond)

	wg.Add(2)
	go func() {
		broker.Publish(ctx, testMessage)
	}()

	wg.Wait()
	cancel(nil)
}
