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

var _ Locker = &memoryLocker{}

func NewMemoryLocker() Locker {
	return &memoryLocker{}
}

func (l *memoryLocker) Lock(ctx context.Context, key string) (context.Context, ReleaseFunc, error) {
	originalCtx := ctx

	if l.tryLock(key) {
		ctx, cancel := context.WithCancelCause(ctx)
		releaseOnce := sync.Once{}
		return ctx, func() context.Context {
			releaseOnce.Do(func() {
				l.locks.Delete(key)
				cancel(ErrLockReleased)
			})
			return originalCtx
		}, nil
	}

	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx, func() context.Context { return originalCtx }, ctx.Err()
		case <-ticker.C:
			if l.tryLock(key) {
				ctx, cancel := context.WithCancelCause(ctx)
				releaseOnce := sync.Once{}
				return ctx, func() context.Context {
					releaseOnce.Do(func() {
						l.locks.Delete(key)
						cancel(ErrLockReleased)
					})
					return originalCtx
				}, nil
			}
		}
	}
}

func (l *memoryLocker) TryLock(ctx context.Context, key string) (bool, context.Context, ReleaseFunc, error) {
	originalCtx := ctx

	if l.tryLock(key) {
		ctx, cancel := context.WithCancelCause(ctx)
		releaseOnce := sync.Once{}
		return true, ctx, func() context.Context {
			releaseOnce.Do(func() {
				cancel(ErrLockReleased)
				l.locks.Delete(key)
			})
			return originalCtx
		}, nil
	}

	return false, ctx, func() context.Context { return originalCtx }, nil
}

func (l *memoryLocker) tryLock(key string) bool {
	_, loaded := l.locks.LoadOrStore(key, struct{}{})
	return !loaded
}
