// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package globallock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4"
)

type redisLocker struct {
	rs *redsync.Redsync

	mutexM sync.Map
}

func (l *redisLocker) Lock(ctx context.Context, key string) (context.Context, func(), error) {
	return l.lock(ctx, key, 0)
}

func (l *redisLocker) TryLock(ctx context.Context, key string) (bool, context.Context, func(), error) {
	ctx, f, err := l.lock(ctx, key, 1)
	return err == nil, ctx, f, err
}

type redisMutex struct {
	mutex  *redsync.Mutex
	cancel context.CancelCauseFunc
}

func (l *redisLocker) lock(ctx context.Context, key string, tries int) (context.Context, func(), error) {
	var options []redsync.Option
	if tries > 0 {
		options = append(options, redsync.WithTries(tries))
	}
	mutex := l.rs.NewMutex(key, options...)
	if err := mutex.LockContext(ctx); err != nil {
		return ctx, func() {}, err
	}

	ctx, cancel := context.WithCancelCause(ctx)

	l.mutexM.Store(key, &redisMutex{
		mutex:  mutex,
		cancel: cancel,
	})

	return ctx, func() {
		l.mutexM.Delete(key)

		// It's safe to ignore the error here,
		// if the lock is not released, it will be released automatically after the lock expires.
		// Do not call mutex.UnlockContext(ctx) here, or it will fail to unlock when ctx has timed out.
		_, _ = mutex.Unlock()
		cancel(fmt.Errorf("lock released"))
	}, nil
}

func (l *redisLocker) extend() {
	toExtend := make([]*redisMutex, 0)
	l.mutexM.Range(func(_, value interface{}) bool {
		m := value.(*redisMutex)

		// Extend the lock if it is not expired.
		// Although the mutex will be removed from the map before it is unlocked,
		// it still can be expired because of a failed extension.
		// If it happens, the cancel function should have been called,
		// so it does not need to be extended anymore.
		if time.Now().Before(m.mutex.Until()) {
			toExtend = append(toExtend, m)
		}
		return true
	})
	for _, v := range toExtend {
		if ok, err := v.mutex.Extend(); !ok {
			v.cancel(err)
		}
	}

	// TODO: a better duration
	time.AfterFunc(5*time.Second, l.extend)
}
