// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"time"
)

var pushBlockTime = 5 * time.Second

type baseQueue interface {
	PushItem(ctx context.Context, data []byte) error
	PopItem(ctx context.Context) ([]byte, error)
	HasItem(ctx context.Context, data []byte) (bool, error)
	Len(ctx context.Context) (int, error)
	Close() error
	RemoveAll(ctx context.Context) error
}

func popItemByChan(ctx context.Context, popItemFn func(ctx context.Context) ([]byte, error)) (chanItem chan []byte, chanErr chan error) {
	chanItem = make(chan []byte)
	chanErr = make(chan error)
	go func() {
		for {
			it, err := popItemFn(ctx)
			if err != nil {
				close(chanItem)
				chanErr <- err
				return
			}
			if it == nil {
				close(chanItem)
				close(chanErr)
				return
			}
			chanItem <- it
		}
	}()
	return chanItem, chanErr
}
