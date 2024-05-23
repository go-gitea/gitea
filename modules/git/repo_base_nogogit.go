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

	tagCache *ObjectCache

	gpgSettings *GPGSettings

	batchInUse  bool
	batchCancel context.CancelFunc
	batchReader *bufio.Reader
	batchWriter WriteCloserError

	checkInUse  bool
	checkCancel context.CancelFunc
	checkReader *bufio.Reader
	checkWriter WriteCloserError

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

	// Now because of some insanity with git cat-file not immediately failing if not run in a valid git directory we need to run git rev-parse first!
	if err := EnsureValidGitRepository(ctx, repoPath); err != nil {
		return nil, err
	}

	repo := &Repository{
		Path:     repoPath,
		tagCache: newObjectCache(),
		Ctx:      ctx,
	}

	repo.batchWriter, repo.batchReader, repo.batchCancel = CatFileBatch(ctx, repoPath)
	repo.checkWriter, repo.checkReader, repo.checkCancel = CatFileBatchCheck(ctx, repoPath)

	return repo, nil
}

// CatFileBatch obtains a CatFileBatch for this repository
func (repo *Repository) CatFileBatch(ctx context.Context) (WriteCloserError, *bufio.Reader, func()) {
	if repo.batchCancel == nil || repo.batchInUse {
		log.Debug("Opening temporary cat file batch for: %s", repo.Path)
		return CatFileBatch(ctx, repo.Path)
	}
	repo.batchInUse = true
	return repo.batchWriter, repo.batchReader, func() {
		repo.batchInUse = false
	}
}

// CatFileBatchCheck obtains a CatFileBatchCheck for this repository
func (repo *Repository) CatFileBatchCheck(ctx context.Context) (WriteCloserError, *bufio.Reader, func()) {
	if repo.checkCancel == nil || repo.checkInUse {
		log.Debug("Opening temporary cat file batch-check for: %s", repo.Path)
		return CatFileBatchCheck(ctx, repo.Path)
	}
	repo.checkInUse = true
	return repo.checkWriter, repo.checkReader, func() {
		repo.checkInUse = false
	}
}

func (repo *Repository) Close() error {
	if repo == nil {
		return nil
	}
	if repo.batchCancel != nil {
		repo.batchCancel()
		repo.batchReader = nil
		repo.batchWriter = nil
		repo.batchCancel = nil
		repo.batchInUse = false
	}
	if repo.checkCancel != nil {
		repo.checkCancel()
		repo.checkCancel = nil
		repo.checkReader = nil
		repo.checkWriter = nil
		repo.checkInUse = false
	}
	repo.LastCommitCache = nil
	repo.tagCache = nil
	return nil
}
