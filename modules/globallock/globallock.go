// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package globallock

import (
	"context"
	"sync"

	"code.gitea.io/gitea/modules/setting"
)

var (
	defaultLocker Locker
	initOnce      sync.Once
	initFunc      = func() {
		switch setting.GlobalLock.ServiceType {
		case "redis":
			defaultLocker = NewRedisLocker(setting.GlobalLock.ServiceConnStr)
		case "memory":
			fallthrough
		default:
			defaultLocker = NewMemoryLocker()
		}
	} // define initFunc as a variable to make it possible to change it in tests
)

// DefaultLocker returns the default locker.
func DefaultLocker() Locker {
	initOnce.Do(func() {
		initFunc()
	})
	return defaultLocker
}

// Lock tries to acquire a lock for the given key, it uses the default locker.
// Read the documentation of Locker.Lock for more information about the behavior.
func Lock(ctx context.Context, key string) (ReleaseFunc, error) {
	return DefaultLocker().Lock(ctx, key)
}

// TryLock tries to acquire a lock for the given key, it uses the default locker.
// Read the documentation of Locker.TryLock for more information about the behavior.
func TryLock(ctx context.Context, key string) (bool, ReleaseFunc, error) {
	return DefaultLocker().TryLock(ctx, key)
}

// LockAndDo tries to acquire a lock for the given key and then calls the given function.
// It uses the default locker, and it will return an error if failed to acquire the lock.
func LockAndDo(ctx context.Context, key string, f func(context.Context) error) error {
	release, err := Lock(ctx, key)
	if err != nil {
		return err
	}
	defer release()

	return f(ctx)
}

// TryLockAndDo tries to acquire a lock for the given key and then calls the given function.
// It uses the default locker, and it will return false if failed to acquire the lock.
func TryLockAndDo(ctx context.Context, key string, f func(context.Context) error) (bool, error) {
	ok, release, err := TryLock(ctx, key)
	if err != nil {
		return false, err
	}
	defer release()

	if !ok {
		return false, nil
	}

	return true, f(ctx)
}
