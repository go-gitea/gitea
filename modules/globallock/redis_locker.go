// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package globallock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/nosql"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
)

const (
	redisLockKeyPrefix = "gitea:globallock:"
	redisLockExpiry    = 30 * time.Second
)

type redisLocker struct {
	rs *redsync.Redsync

	mutexM sync.Map
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
	l.startExtend()

	return l
}

func (l *redisLocker) Lock(ctx context.Context, key string) (context.Context, func(), error) {
	return l.lock(ctx, key, 0)
}

func (l *redisLocker) TryLock(ctx context.Context, key string) (bool, context.Context, func(), error) {
	ctx, f, err := l.lock(ctx, key, 1)
	if errors.Is(err, redsync.ErrFailed) {
		return false, ctx, f, nil
	}
	return err == nil, ctx, f, err
}

type redisMutex struct {
	mutex  *redsync.Mutex
	cancel context.CancelCauseFunc
}

func (l *redisLocker) lock(ctx context.Context, key string, tries int) (context.Context, func(), error) {
	options := []redsync.Option{
		redsync.WithExpiry(redisLockExpiry),
	}
	if tries > 0 {
		options = append(options, redsync.WithTries(tries))
	}
	mutex := l.rs.NewMutex(redisLockKeyPrefix+key, options...)
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
		cancel(fmt.Errorf("unlock"))
	}, nil
}

func (l *redisLocker) startExtend() {
	toExtend := make([]*redisMutex, 0)
	l.mutexM.Range(func(_, value interface{}) bool {
		m := value.(*redisMutex)

		// Extend the lock if it is not expired.
		// Although the mutex will be removed from the map before it is unlocked,
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
