// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build !gogit
// +build !gogit

package git

import (
	"bufio"
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"sync"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
)

// Repository represents a Git repository.
type Repository struct {
	Path string

	tagCache *ObjectCache

	gpgSettings *GPGSettings

	lock sync.Mutex

	closed bool

	batchCancel context.CancelFunc
	batchReader *bufio.Reader
	batchWriter WriteCloserError

	checkCancel context.CancelFunc
	checkReader *bufio.Reader
	checkWriter WriteCloserError

	Ctx context.Context
}

// OpenRepository opens the repository at the given path.
func OpenRepository(repoPath string) (*Repository, error) {
	return OpenRepositoryCtx(DefaultContext, repoPath)
}

// OpenRepositoryCtx opens the repository at the given path with the provided context.
func OpenRepositoryCtx(ctx context.Context, repoPath string) (*Repository, error) {
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	} else if !isDir(repoPath) {
		return nil, errors.New("no such file or directory")
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
	repo.checkWriter, repo.checkReader, repo.checkCancel = CatFileBatchCheck(ctx, repo.Path)

	runtime.SetFinalizer(repo, (*Repository).finalizer)

	return repo, nil
}

// CatFileBatch obtains a CatFileBatch for this repository
func (repo *Repository) CatFileBatch(ctx context.Context) (WriteCloserError, *bufio.Reader, func()) {
	repo.lock.Lock()
	defer repo.lock.Unlock()

	if repo.closed || repo.batchReader.Buffered() > 0 {
		log.Debug("Opening temporary cat file batch for: %s", repo.Path)
		return CatFileBatch(ctx, repo.Path)
	}

	if repo.batchCancel == nil {
		repo.batchWriter, repo.batchReader, repo.batchCancel = CatFileBatch(ctx, repo.Path)
	}

	return repo.batchWriter, repo.batchReader, func() {}
}

// CatFileBatchCheck obtains a CatFileBatchCheck for this repository
func (repo *Repository) CatFileBatchCheck(ctx context.Context) (WriteCloserError, *bufio.Reader, func()) {
	repo.lock.Lock()
	defer repo.lock.Unlock()

	if repo.closed || repo.checkReader.Buffered() > 0 {
		log.Debug("Opening temporary cat file batch-check: %s", repo.Path)
		return CatFileBatchCheck(ctx, repo.Path)
	}

	if repo.checkCancel == nil {
		repo.checkWriter, repo.checkReader, repo.checkCancel = CatFileBatchCheck(ctx, repo.Path)
	}

	return repo.checkWriter, repo.checkReader, func() {}
}

// Close this repository, in particular close the underlying gogitStorage if this is not nil
func (repo *Repository) Close() (err error) {
	if repo == nil {
		return
	}
	repo.lock.Lock()
	defer repo.lock.Unlock()

	return repo.close()
}

func (repo *Repository) close() (err error) {
	if repo == nil {
		return
	}
	if repo.batchCancel != nil {
		repo.batchCancel()
		repo.batchReader = nil
		repo.batchWriter = nil
		repo.batchCancel = nil
	}
	if repo.checkCancel != nil {
		repo.checkCancel()
		repo.checkCancel = nil
		repo.checkReader = nil
		repo.checkWriter = nil
	}
	repo.closed = true
	return
}

func (repo *Repository) finalizer() (err error) {
	if repo == nil {
		return
	}
	repo.lock.Lock()
	defer repo.lock.Unlock()
	if repo.closed {
		return nil
	}

	if repo.batchCancel != nil || repo.checkCancel != nil {
		pid := ""
		if repo.Ctx != nil {
			pid = " from PID: " + string(process.GetPID(repo.Ctx))
		}
		log.Error("Finalizer running on unclosed repository%s: %s%s", pid, repo.Path)
	}
	return repo.close()
}
