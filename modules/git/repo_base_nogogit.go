// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"bufio"
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
	batch      *Batch

	checkInUse bool
	check      *Batch

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
func (repo *Repository) CatFileBatch(ctx context.Context) (WriteCloserError, *bufio.Reader, func(), error) {
	if repo.batch == nil {
		var err error
		repo.batch, err = repo.NewBatch(ctx)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	if !repo.batchInUse {
		repo.batchInUse = true
		return repo.batch.Writer, repo.batch.Reader, func() {
			repo.batchInUse = false
		}, nil
	}

	log.Debug("Opening temporary cat file batch for: %s", repo.Path)
	tempBatch, err := repo.NewBatch(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	return tempBatch.Writer, tempBatch.Reader, tempBatch.Close, nil
}

// CatFileBatchCheck obtains a CatFileBatchCheck for this repository
func (repo *Repository) CatFileBatchCheck(ctx context.Context) (WriteCloserError, *bufio.Reader, func(), error) {
	if repo.check == nil {
		var err error
		repo.check, err = repo.NewBatchCheck(ctx)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	if !repo.checkInUse {
		repo.checkInUse = true
		return repo.check.Writer, repo.check.Reader, func() {
			repo.checkInUse = false
		}, nil
	}

	log.Debug("Opening temporary cat file batch-check for: %s", repo.Path)
	tempBatchCheck, err := repo.NewBatchCheck(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	return tempBatchCheck.Writer, tempBatchCheck.Reader, tempBatchCheck.Close, nil
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
	if repo.check != nil {
		repo.check.Close()
		repo.check = nil
		repo.checkInUse = false
	}
	repo.LastCommitCache = nil
	repo.tagCache = nil
	return nil
}
