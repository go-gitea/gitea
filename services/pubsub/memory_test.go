// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"context"
	"sync"
	"testing"

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
	broker.Subscribe(ctx, testTopic, func(message []byte) { assert.Equal(t, testMessage, message); wg.Done() })
	broker.Subscribe(ctx, testTopic, func(_ []byte) { wg.Done() })

	wg.Add(2)
	broker.Publish(ctx, testTopic, testMessage)

	wg.Wait()
	cancel(nil)
}
