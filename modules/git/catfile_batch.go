// Copyright 2026 The Gitea Authors. All rights reserved.
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

type CatFileObject struct {
	ID   string
	Type string
	Size int64
}

type CatFileBatchContent interface {
	// QueryContent queries the object info with content from the git repository
	// FIXME: this design still follows the old pattern: the returned BufferedReader is very fragile, callers should carefully maintain its lifecycle and discard all unread data
	// It needs to refactor to a fully managed Reader stream in the future
	QueryContent(obj string) (*CatFileObject, BufferedReader, error)
}

type CatFileBatchInfo interface {
	QueryInfo(obj string) (*CatFileObject, error)
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
	if DefaultFeatures().SupportCatFileBatchCommand {
		return newCatFileBatchCommand(ctx, repoPath)
	}
	return newCatFileBatchLegacy(ctx, repoPath)
}
