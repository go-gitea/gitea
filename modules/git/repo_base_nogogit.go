// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"
	"path/filepath"

	"code.gitea.io/gitea/modules/git/catfile"
	"code.gitea.io/gitea/modules/util"
)

const isGogit = false

// Repository represents a Git repository.
type Repository struct {
	Path string

	tagCache *ObjectCache[*Tag]

	objectPool catfile.ObjectPool

	Ctx             context.Context
	LastCommitCache *LastCommitCache

	objectFormat ObjectFormat
}

// OpenRepository opens the repository at the given path with the provided context.
func OpenRepository(ctx context.Context, repoPath string) (*Repository, error) {
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}
	exist, err := util.IsDir(repoPath)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, util.NewNotExistErrorf("no such file or directory")
	}

	objectPool, err := catfile.NewBatchObjectPool(ctx, repoPath)
	if err != nil {
		return nil, err
	}

	return &Repository{
		Path:       repoPath,
		tagCache:   newObjectCache[*Tag](),
		objectPool: objectPool,
		Ctx:        ctx,
	}, nil
}

func (repo *Repository) ObjectPool() catfile.ObjectPool {
	return repo.objectPool
}

func (repo *Repository) Close() error {
	if repo == nil {
		return nil
	}
	if repo.objectPool != nil {
		repo.objectPool.Close()
	}
	repo.LastCommitCache = nil
	repo.tagCache = nil
	return nil
}
