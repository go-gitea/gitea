// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package globallock

import (
	"context"
	"sync"
	"time"
)

type memoryLocker struct {
	locks sync.Map
}

func (l *memoryLocker) Lock(ctx context.Context, key string) (context.Context, func(), error) {
	if l.tryLock(key) {
		return ctx, func() {
			l.locks.Delete(key)
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
				return ctx, func() {
					l.locks.Delete(key)
				}, nil
			}
		}
	}
}

func (l *memoryLocker) TryLock(ctx context.Context, key string) (bool, context.Context, func(), error) {
	if l.tryLock(key) {
		return true, ctx, func() {
			l.locks.Delete(key)
		}, nil
	}
	return false, ctx, func() {}, nil
}

func (l *memoryLocker) tryLock(key string) bool {
	_, loaded := l.locks.LoadOrStore(key, struct{}{})
	return !loaded
}
