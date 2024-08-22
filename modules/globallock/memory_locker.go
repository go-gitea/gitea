// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package globallock

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type memoryLocker struct {
	locks sync.Map
}

var _ Locker = &memoryLocker{}

func NewMemoryLocker() Locker {
	return &memoryLocker{}
}

func (l *memoryLocker) Lock(ctx context.Context, key string) (context.Context, func(), error) {
	if l.tryLock(key) {
		ctx, cancel := context.WithCancelCause(ctx)
		return ctx, func() {
			l.locks.Delete(key)
			cancel(fmt.Errorf("unlock"))
		}, nil
	}

	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx, func() {}, ctx.Err()
		case <-ticker.C:
			if l.tryLock(key) {
				ctx, cancel := context.WithCancelCause(ctx)
				return ctx, func() {
					l.locks.Delete(key)
					cancel(fmt.Errorf("unlock"))
				}, nil
			}
		}
	}
}

func (l *memoryLocker) TryLock(ctx context.Context, key string) (bool, context.Context, func(), error) {
	if l.tryLock(key) {
		ctx, cancel := context.WithCancelCause(ctx)
		return true, ctx, func() {
			cancel(fmt.Errorf("unlock"))
			l.locks.Delete(key)
		}, nil
	}

	return false, ctx, func() {}, nil
}

func (l *memoryLocker) tryLock(key string) bool {
	_, loaded := l.locks.LoadOrStore(key, struct{}{})
	return !loaded
}
