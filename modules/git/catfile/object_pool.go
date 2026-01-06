// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package catfile

import (
	"io"
)

type ObjectInfo struct {
	ID   string
	Type string
	Size int64
}

type Discarder interface {
	Discard(n int) (int, error)
}

type ReadCloseDiscarder interface {
	io.ReadCloser
	Discarder
	ReadBytes(delim byte) ([]byte, error)
	ReadSlice(delim byte) (line []byte, err error)
}

type ObjectPool interface {
	ObjectInfo(refName string) (*ObjectInfo, error)
	Object(refName string) (*ObjectInfo, ReadCloseDiscarder, error)
	Close()
}
