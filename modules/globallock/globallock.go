// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package globallock

import (
	"context"
	"fmt"
)

type Locker interface {
	Lock(ctx context.Context, key string) (context.Context, ReleaseFunc, error)
	TryLock(ctx context.Context, key string) (bool, context.Context, ReleaseFunc, error)
}

type ReleaseFunc func() context.Context

// ErrLockReleased is used as context cause when a lock is released
var ErrLockReleased = fmt.Errorf("lock released")
