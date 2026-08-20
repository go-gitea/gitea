// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"time"
)

// leveldb and redis have no blocking pop, so their PopItem waits here
var pollFallbackInterval = 2 * time.Second

// one pending signal is enough, the woken reader keeps popping until the queue is empty
func signalPush(pushed chan struct{}) {
	select {
	case pushed <- struct{}{}:
	default:
	}
}

func waitForPush(ctx context.Context, pushed <-chan struct{}) error {
	select {
	case <-pushed:
	case <-time.After(pollFallbackInterval): // catches a push that did not signal us, e.g. from another process
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
