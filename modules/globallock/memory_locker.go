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

func (l *memoryLocker) Lock(ctx context.Context, key string) (ReleaseFunc, error) {
	if l.tryLock(key) {
		releaseOnce := sync.Once{}
		return func() {
			releaseOnce.Do(func() {
				l.locks.Delete(key)
			})
		}, nil
	}

	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return func() {}, ctx.Err()
		case <-ticker.C:
			if l.tryLock(key) {
				releaseOnce := sync.Once{}
				return func() {
					releaseOnce.Do(func() {
						l.locks.Delete(key)
					})
				}, nil
			}
		}
	}
}

func (l *memoryLocker) TryLock(_ context.Context, key string) (bool, ReleaseFunc, error) {
	if l.tryLock(key) {
		releaseOnce := sync.Once{}
		return true, func() {
			releaseOnce.Do(func() {
				l.locks.Delete(key)
			})
		}, nil
	}

	return false, func() {}, nil
}

func (l *memoryLocker) tryLock(key string) bool {
	_, loaded := l.locks.LoadOrStore(key, struct{}{})
	return !loaded
}
