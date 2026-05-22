// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package globallock

import (
	"context"
	"sync"
	"sync/atomic"

	"code.gitea.io/gitea/modules/setting"
)

var (
	defaultLocker atomic.Pointer[Locker]
	defaultMutex  sync.Mutex
)

func initDefaultLocker() Locker {
	switch setting.GlobalLock.ServiceType {
	case "redis":
		return NewRedisLocker(setting.GlobalLock.ServiceConnStr)
	default: // "memory"
		return NewMemoryLocker()
	}
}

// DefaultLocker returns the default locker.
func DefaultLocker() Locker {
	ptr := defaultLocker.Load()
	if ptr == nil {
		defaultMutex.Lock()
		ptr = defaultLocker.Load()
		if ptr == nil {
			ptr = new(initDefaultLocker())
			defaultLocker.Store(ptr)
		}
		defaultMutex.Unlock()
		ptr = defaultLocker.Load()
	}
	return *ptr
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
