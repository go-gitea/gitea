// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"io"
)

type BufferedReader interface {
	io.Reader
	Buffered() int
	Peek(n int) ([]byte, error)
	Discard(n int) (int, error)
	ReadString(sep byte) (string, error)
	ReadSlice(sep byte) ([]byte, error)
	ReadBytes(sep byte) ([]byte, error)
}

type CatFileBatchContent interface {
	QueryContent(obj string) (BufferedReader, error)
}

type CatFileBatchInfo interface {
	QueryInfo(obj string) (BufferedReader, error)
}

type CatFileBatch interface {
	CatFileBatchInfo
	CatFileBatchContent
}

type CatFileBatchCloser interface {
	CatFileBatch
	Close()
}

func NewBatch(ctx context.Context, repoPath string) (CatFileBatchCloser, error) {
	return newCatFileBatchLegacy(ctx, repoPath)
}
