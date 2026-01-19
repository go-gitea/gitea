// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package objectpool

import "context"

type Provider interface {
	GetObjectPool(ctx context.Context) (ObjectPool, func(), error)
	Close()
}
