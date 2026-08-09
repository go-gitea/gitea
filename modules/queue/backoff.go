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

func backoffErr(ctx context.Context, begin, upper time.Duration, end <-chan time.Time, fn func() (retry bool, err error)) error {
	d := begin
	for {
		// check whether the context has been cancelled or has reached the deadline, return early
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-end:
			return context.DeadlineExceeded
		default:
		}

		// call the target function
		retry, err := fn()
		if err != nil {
			return err
		}
		if !retry {
			return nil
		}

		// wait for a while before retrying, and also respect the context & deadline
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
			d *= 2
			if d > upper {
				d = upper
			}
		case <-end:
			return context.DeadlineExceeded
		}
	}
}
