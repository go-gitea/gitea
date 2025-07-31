// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

const isGogit = false

// Repository represents a Git repository.
type Repository struct {
	Path string

	tagCache *ObjectCache[*Tag]

	gpgSettings *GPGSettings

	batchInUse bool
	batch      Batch

	Ctx             context.Context
	LastCommitCache *LastCommitCache

	objectFormat ObjectFormat
}

// openRepositoryWithDefaultContext opens the repository at the given path with DefaultContext.
func openRepositoryWithDefaultContext(repoPath string) (*Repository, error) {
	return OpenRepository(DefaultContext, repoPath)
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
		Path:     repoPath,
		tagCache: newObjectCache[*Tag](),
		Ctx:      ctx,
	}, nil
}

// CatFileBatch obtains a CatFileBatch for this repository
func (repo *Repository) CatFileBatch(ctx context.Context) (Batch, func(), error) {
	if repo.batch == nil {
		var err error
		repo.batch, err = NewBatch(ctx, repo.Path)
		if err != nil {
			return nil, nil, err
		}
	}

	if !repo.batchInUse {
		repo.batchInUse = true
		return repo.batch, func() {
			repo.batchInUse = false
		}, nil
	}

	log.Debug("Opening temporary cat file batch for: %s", repo.Path)
	tempBatch, err := NewBatch(ctx, repo.Path)
	if err != nil {
		return nil, nil, err
	}
	return tempBatch, tempBatch.Close, nil
}

func (repo *Repository) Close() error {
	if repo == nil {
		return nil
	}
	if repo.batch != nil {
		repo.batch.Close()
		repo.batch = nil
		repo.batchInUse = false
	}

	repo.LastCommitCache = nil
	repo.tagCache = nil
	return nil
}
