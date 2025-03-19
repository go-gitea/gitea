// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"
	"path/filepath"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

const isGogit = false

// Repository represents a Git repository.
type Repository struct {
	Path string

	tagCache *ObjectCache[*Tag]

	gpgSettings *GPGSettings

	batchInUse bool
	batch      *BatchCatFile

	checkInUse bool
	batchCheck *BatchCatFile

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
	} else if !isDir(repoPath) {
		return nil, util.NewNotExistErrorf("no such file or directory")
	}

	return &Repository{
		Path:     repoPath,
		tagCache: newObjectCache[*Tag](),
		Ctx:      ctx,
	}, nil
}

// CatFileBatch obtains a CatFileBatch for this repository
func (repo *Repository) CatFileBatch(ctx context.Context) (*BatchCatFile, func(), error) {
	if repo.batch == nil {
		var err error
		repo.batch, err = NewBatchCatFile(ctx, repo.Path, false)
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

	if !setting.DisableTempCatFileBatchCheck {
		setting.PanicInDevOrTesting("Opening temporary cat file batch for: %s", repo.Path)
	}
	tempBatch, err := NewBatchCatFile(ctx, repo.Path, false)
	if err != nil {
		return nil, nil, err
	}
	return tempBatch, func() {
		_ = tempBatch.Close()
	}, nil
}

// CatFileBatchCheck obtains a CatFileBatchCheck for this repository
func (repo *Repository) CatFileBatchCheck(ctx context.Context) (*BatchCatFile, func(), error) {
	if repo.batchCheck == nil {
		var err error
		repo.batchCheck, err = NewBatchCatFile(ctx, repo.Path, true)
		if err != nil {
			return nil, nil, err
		}
	}

	if !repo.checkInUse {
		repo.checkInUse = true
		return repo.batchCheck, func() {
			repo.checkInUse = false
		}, nil
	}

	setting.PanicInDevOrTesting("Opening temporary cat file batch with check for: %s", repo.Path)
	tempBatch, err := NewBatchCatFile(ctx, repo.Path, true)
	if err != nil {
		return nil, nil, err
	}
	return tempBatch, func() {
		_ = tempBatch.Close()
	}, nil
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
	if repo.batchCheck != nil {
		repo.batchCheck.Close()
		repo.batchCheck = nil
		repo.checkInUse = false
	}
	repo.LastCommitCache = nil
	repo.tagCache = nil
	return nil
}
