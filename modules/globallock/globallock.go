// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package globallock

import (
	"context"
)

type Locker interface {
	Lock(ctx context.Context, key string) (context.Context, func(), error)
	TryLock(ctx context.Context, key string) (bool, context.Context, func(), error)
}
