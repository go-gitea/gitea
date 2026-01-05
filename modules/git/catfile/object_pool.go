// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package catfile

import (
	"bufio"
	"context"
)

type ObjectInfo struct {
	ID   string
	Type string
	Size int64
}

type ObjectInfoPool interface {
	ObjectInfo(ctx context.Context, sha string) (*ObjectInfo, error)
	Close()
}

type Object struct {
	ObjectInfo
	Reader *bufio.Reader
}

type ObjectPool interface {
	Object(ctx context.Context, sha string) (*Object, error)
	Close()
}
