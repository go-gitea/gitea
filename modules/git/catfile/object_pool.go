// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package catfile

import (
	"bufio"
)

type ObjectInfo struct {
	ID   string
	Type string
	Size int64
}

type ObjectInfoPool interface {
	ObjectInfo(refName string) (*ObjectInfo, error)
	Close()
}

type ObjectPool interface {
	Object(refName string) (*ObjectInfo, *bufio.Reader, error)
	Close()
}
