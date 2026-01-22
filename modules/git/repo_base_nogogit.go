// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"
	"path/filepath"

	"code.gitea.io/gitea/modules/git/objectpool"
	"code.gitea.io/gitea/modules/util"
)

const isGogit = false

// Repository represents a Git repository.
type Repository struct {
	Path string

	tagCache *ObjectCache[*Tag]

	objPoolProvider objectpool.Provider

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

	return &Repository{
		Path:            repoPath,
		tagCache:        newObjectCache[*Tag](),
		objPoolProvider: NewObjectPoolProvider(repoPath),
		Ctx:             ctx,
	}, nil
}

// GetObjectPool obtains a "batch object provider" for this repository.
// It reuses an existing one if available, otherwise creates a new one.
func (repo *Repository) GetObjectPool(ctx context.Context) (_ objectpool.ObjectPool, closeFunc func(), err error) {
	return repo.objPoolProvider.GetObjectPool(ctx)
}

func (repo *Repository) Close() error {
	if repo == nil {
		return nil
	}
	if repo.objPoolProvider != nil {
		repo.objPoolProvider.Close()
		repo.objPoolProvider = nil
	}
	repo.LastCommitCache = nil
	repo.tagCache = nil
	return nil
}
