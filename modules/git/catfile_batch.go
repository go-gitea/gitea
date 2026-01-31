// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"io"
)

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

	// QueryContent is similar to QueryInfo. It provides the object info and a reader for its content to the handler.
	// The reader is limited to the object's size, and remaining bytes are discarded after the handler returns.
	// The handler should not retain the reader or invoke QueryContent recursively.
	QueryContent(obj string, handler func(info *CatFileObject, reader io.Reader) error) error
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
