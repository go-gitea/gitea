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

type backoffFunc[T any] func() (retry bool, ret T, err error)

type backoffOptions struct {
	begin, upper time.Duration

	notify <-chan struct{}
	end    <-chan time.Time
}

func backoffOptionsDefault(notify <-chan struct{}, end <-chan time.Time) backoffOptions {
	return backoffOptions{begin: backoffBegin, upper: backoffUpper, notify: notify, end: end}
}

func mockBackoffDuration(d time.Duration) func() {
	oldBegin, oldUpper := backoffBegin, backoffUpper
	backoffBegin, backoffUpper = d, d
	return func() {
		backoffBegin, backoffUpper = oldBegin, oldUpper
	}
}

func backoffCall[T any](ctx context.Context, opts backoffOptions, fn backoffFunc[T]) (ret T, err error) {
	d := opts.begin
	for {
		// call the target function
		retry, ret, err := fn()
		if err != nil {
			return ret, err
		}
		if !retry {
			return ret, nil
		}

		// wait for a while before retrying, and also respect the context & deadline & notify
		select {
		case <-ctx.Done():
			return ret, ctx.Err()
		case <-opts.end:
			return ret, context.DeadlineExceeded
		case <-opts.notify:
			continue
		case <-time.After(d):
			d *= 2
			if d > opts.upper {
				d = opts.upper
			}
		}
	}
}
