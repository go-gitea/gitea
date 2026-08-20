// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"time"
)

var (
	backoffBegin = 50 * time.Millisecond
	backoffUpper = 2 * time.Second
)

func mockBackoffDuration(d time.Duration) func() {
	oldBegin, oldUpper := backoffBegin, backoffUpper
	backoffBegin, backoffUpper = d, d
	return func() {
		backoffBegin, backoffUpper = oldBegin, oldUpper
	}
}

// backoffErr retries fn until it succeeds or pushBlockTime elapses.
func backoffErr(ctx context.Context, fn func() (retry bool, err error)) error {
	d, end := backoffBegin, time.After(pushBlockTime)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-end:
			return context.DeadlineExceeded
		default:
		}

		retry, err := fn()
		if err != nil {
			return err
		}
		if !retry {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
			d = min(d*2, backoffUpper)
		case <-end:
			return context.DeadlineExceeded
		}
	}
}
