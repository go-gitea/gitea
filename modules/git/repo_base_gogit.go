// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build gogit
// +build gogit

package git

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"sync"

	"code.gitea.io/gitea/modules/log"
	gitealog "code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"

	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// Repository represents a Git repository.
type Repository struct {
	Path string

	tagCache *ObjectCache

	lock   sync.Mutex
	closed bool

	gogitRepo    *gogit.Repository
	gogitStorage *filesystem.Storage
	gpgSettings  *GPGSettings

	Ctx context.Context
}

// OpenRepository opens the repository at the given path.
func OpenRepository(repoPath string) (*Repository, error) {
	return OpenRepositoryCtx(DefaultContext, repoPath)
}

// OpenRepositoryCtx opens the repository at the given path within the context.Context
func OpenRepositoryCtx(ctx context.Context, repoPath string) (*Repository, error) {
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	} else if !isDir(repoPath) {
		return nil, errors.New("no such file or directory")
	}

	fs := osfs.New(repoPath)
	_, err = fs.Stat(".git")
	if err == nil {
		fs, err = fs.Chroot(".git")
		if err != nil {
			return nil, err
		}
	}
	storage := filesystem.NewStorageWithOptions(fs, cache.NewObjectLRUDefault(), filesystem.Options{KeepDescriptors: true, LargeObjectThreshold: setting.Git.LargeObjectThreshold})
	gogitRepo, err := gogit.Open(storage, fs)
	if err != nil {
		return nil, err
	}

	repo := &Repository{
		Path:         repoPath,
		gogitRepo:    gogitRepo,
		gogitStorage: storage,
		tagCache:     newObjectCache(),
		Ctx:          ctx,
	}

	runtime.SetFinalizer(repo, (*Repository).finalizer)

	return repo, nil
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
	repo.closed = true
	if repo.gogitStorage == nil {
		return
	}
	err = repo.gogitStorage.Close()
	if err != nil {
		gitealog.Error("Error closing storage: %v", err)
	}
	return
}

func (repo *Repository) finalizer() error {
	if repo == nil {
		return nil
	}
	repo.lock.Lock()
	defer repo.lock.Unlock()
	if !repo.closed {
		pid := ""
		if repo.Ctx != nil {
			pid = " from PID: " + string(process.GetPID(repo.Ctx))
		}
		log.Error("Finalizer running on unclosed repository%s: %s%s", pid, repo.Path)
	}

	// We still need to run the close fn as it may be possible to reopen the gogitrepo after close
	return repo.close()
}

// GoGitRepo gets the go-git repo representation
func (repo *Repository) GoGitRepo() *gogit.Repository {
	return repo.gogitRepo
}
