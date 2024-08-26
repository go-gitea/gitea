// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package globallock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/modules/nosql"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
)

const redisLockKeyPrefix = "gitea:globallock:"

// redisLockExpiry is the default expiry time for a lock.
// Define it as a variable to make it possible to change it in tests.
var redisLockExpiry = 30 * time.Second

type redisLocker struct {
	rs *redsync.Redsync

	mutexM   sync.Map
	closed   atomic.Bool
	extendWg sync.WaitGroup
}

var _ Locker = &redisLocker{}

func NewRedisLocker(connection string) Locker {
	l := &redisLocker{
		rs: redsync.New(
			goredis.NewPool(
				nosql.GetManager().GetRedisClient(connection),
			),
		),
	}

	l.extendWg.Add(1)
	l.startExtend()

	return l
}

func (l *redisLocker) Lock(ctx context.Context, key string) (context.Context, ReleaseFunc, error) {
	return l.lock(ctx, key, 0)
}

func (l *redisLocker) TryLock(ctx context.Context, key string) (bool, context.Context, ReleaseFunc, error) {
	ctx, f, err := l.lock(ctx, key, 1)

	var (
		errTaken     *redsync.ErrTaken
		errNodeTaken *redsync.ErrNodeTaken
	)
	if errors.As(err, &errTaken) || errors.As(err, &errNodeTaken) {
		return false, ctx, f, nil
	}
	return err == nil, ctx, f, err
}

// Close closes the locker.
// It will stop extending the locks and refuse to acquire new locks.
// In actual use, it is not necessary to call this function.
// But it's useful in tests to release resources.
// It could take some time since it waits for the extending goroutine to finish.
func (l *redisLocker) Close() error {
	l.closed.Store(true)
	l.extendWg.Wait()
	return nil
}

type redisMutex struct {
	mutex  *redsync.Mutex
	cancel context.CancelCauseFunc
}

func (l *redisLocker) lock(ctx context.Context, key string, tries int) (context.Context, ReleaseFunc, error) {
	if l.closed.Load() {
		return ctx, func() context.Context { return ctx }, fmt.Errorf("locker is closed")
	}

	originalCtx := ctx

	options := []redsync.Option{
		redsync.WithExpiry(redisLockExpiry),
	}
	if tries > 0 {
		options = append(options, redsync.WithTries(tries))
	}
	mutex := l.rs.NewMutex(redisLockKeyPrefix+key, options...)
	if err := mutex.LockContext(ctx); err != nil {
		return ctx, func() context.Context { return originalCtx }, err
	}

	ctx, cancel := context.WithCancelCause(ctx)

	l.mutexM.Store(key, &redisMutex{
		mutex:  mutex,
		cancel: cancel,
	})

	releaseOnce := sync.Once{}
	return ctx, func() context.Context {
		releaseOnce.Do(func() {
			l.mutexM.Delete(key)

			// It's safe to ignore the error here,
			// if it failed to unlock, it will be released automatically after the lock expires.
			// Do not call mutex.UnlockContext(ctx) here, or it will fail to release when ctx has timed out.
			_, _ = mutex.Unlock()

			cancel(ErrLockReleased)
		})
		return originalCtx
	}, nil
}

func (l *redisLocker) startExtend() {
	if l.closed.Load() {
		l.extendWg.Done()
		return
	}

	toExtend := make([]*redisMutex, 0)
	l.mutexM.Range(func(_, value any) bool {
		m := value.(*redisMutex)

		// Extend the lock if it is not expired.
		// Although the mutex will be removed from the map before it is released,
		// it still can be expired because of a failed extension.
		// If it happens, the cancel function should have been called,
		// so it does not need to be extended anymore.
		if time.Now().After(m.mutex.Until()) {
			return true
		}

		toExtend = append(toExtend, m)
		return true
	})
	for _, v := range toExtend {
		if ok, err := v.mutex.Extend(); !ok {
			v.cancel(err)
		}
	}

	time.AfterFunc(redisLockExpiry/2, l.startExtend)
}
