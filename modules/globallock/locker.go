// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package globallock

import (
	"context"
)

type Locker interface {
	// Lock tries to acquire a lock for the given key, it blocks until the lock is acquired or the context is canceled.
	//
	// Lock returns a ReleaseFunc to release the lock, it cannot be nil.
	// It's always safe to call this function even if it fails to acquire the lock, and it will do nothing in that case.
	// And it's also safe to call it multiple times, but it will only release the lock once.
	// That's why it's called ReleaseFunc, not UnlockFunc.
	// But be aware that it's not safe to not call it at all; it could lead to a memory leak.
	// So a recommended pattern is to use defer to call it:
	//   release, err := locker.Lock(ctx, "key")
	//   if err != nil {
	//     return err
	//   }
	//   defer release()
	//
	// Lock returns an error if failed to acquire the lock.
	// Be aware that even the context is not canceled, it's still possible to fail to acquire the lock.
	// For example, redis is down, or it reached the maximum number of tries.
	Lock(ctx context.Context, key string) (ReleaseFunc, error)

	// TryLock tries to acquire a lock for the given key, it returns immediately.
	// It follows the same pattern as Lock, but it doesn't block.
	// And if it fails to acquire the lock because it's already locked, not other reasons like redis is down,
	// it will return false without any error.
	TryLock(ctx context.Context, key string) (bool, ReleaseFunc, error)
}

// ReleaseFunc is a function that releases a lock.
type ReleaseFunc func()
