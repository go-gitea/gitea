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

type CatFileBatch interface {
	// QueryInfo queries the object info from the git repository by its object name using "git cat-file --batch" family commands.
	// "git cat-file" accepts "<rev>" for the object name, it can be a ref name, object id, etc. https://git-scm.com/docs/gitrevisions
	// In Gitea, we only use the simple ref name or object id, no other complex rev syntax like "suffix" or "git describe" although they are supported by git.
	QueryInfo(obj string) (*CatFileObject, error)

	// QueryContent is similar to QueryInfo, it queries the object info and additionally returns a reader for its content.
	// FIXME: this design still follows the old pattern: the returned BufferedReader is very fragile,
	// callers should carefully maintain its lifecycle and discard all unread data.
	// TODO: It needs to be refactored to a fully managed Reader stream in the future, don't let callers manually Close or Discard
	QueryContent(obj string) (*CatFileObject, BufferedReader, error)
}

type CatFileBatchCloser interface {
	CatFileBatch
	Close()
}

// NewBatch creates a "batch object provider (CatFileBatch)" for the given repository path to retrieve object info and content efficiently.
// The CatFileBatch and the readers create by it should only be used in the same goroutine.
func NewBatch(ctx context.Context, repoPath string) (CatFileBatchCloser, error) {
	if DefaultFeatures().SupportCatFileBatchCommand {
		return newCatFileBatchCommand(ctx, repoPath)
	}
	return newCatFileBatchLegacy(ctx, repoPath)
}
