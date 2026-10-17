// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"io"
	"os"

	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/util"
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

	// QueryContent is similar to QueryInfo, it queries the object info and additionally returns a reader for its content.
	// The reader ends with the object's content and is valid until the next query on this batch, which discards any unread content.
	QueryContent(obj string) (*CatFileObject, io.Reader, error)
}

type CatFileBatchCloser interface {
	CatFileBatch
	Context() context.Context
	Close()
}

// NewBatch creates a "batch object provider (CatFileBatch)" for the given repository path to retrieve object info and content efficiently.
// The CatFileBatch and the readers create by it should only be used in the same goroutine.
func NewBatch(ctx context.Context, repo RepositoryFacade) (CatFileBatchCloser, error) {
	repoPath := gitrepo.RepoLocalPath(repo)
	if _, err := os.Stat(repoPath); err != nil {
		return nil, util.NewNotExistErrorf("repo %q doesn't exist", repo.LogString())
	}
	if DefaultFeatures().SupportCatFileBatchCommand {
		return newCatFileBatchCommand(ctx, repo), nil
	}
	return newCatFileBatchLegacy(ctx, repo), nil
}
