// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"context"
	"path/filepath"

	gitealog "code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

const isGogit = true

// Repository represents a Git repository.
type Repository struct {
	Path string

	tagCache *ObjectCache

	gogitRepo    *gogit.Repository
	gogitStorage *filesystem.Storage
	gpgSettings  *GPGSettings

	Ctx             context.Context
	LastCommitCache *LastCommitCache
	objectFormat    ObjectFormat
}

// openRepositoryWithDefaultContext opens the repository at the given path with DefaultContext.
func openRepositoryWithDefaultContext(repoPath string) (*Repository, error) {
	return OpenRepository(DefaultContext, repoPath)
}

// OpenRepository opens the repository at the given path within the context.Context
func OpenRepository(ctx context.Context, repoPath string) (*Repository, error) {
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	} else if !isDir(repoPath) {
		return nil, util.NewNotExistErrorf("no such file or directory")
	}

	fs := osfs.New(repoPath)
	_, err = fs.Stat(".git")
	if err == nil {
		fs, err = fs.Chroot(".git")
		if err != nil {
			return nil, err
		}
	}
	// the "clone --shared" repo doesn't work well with go-git AlternativeFS, https://github.com/go-git/go-git/issues/1006
	// so use "/" for AlternatesFS, I guess it is the same behavior as current nogogit (no limitation or check for the "objects/info/alternates" paths), trust the "clone" command executed by the server.
	var altFs billy.Filesystem
	if setting.IsWindows {
		altFs = osfs.New(filepath.VolumeName(setting.RepoRootPath) + "\\") // TODO: does it really work for Windows? Need some time to check.
	} else {
		altFs = osfs.New("/")
	}
	storage := filesystem.NewStorageWithOptions(fs, cache.NewObjectLRUDefault(), filesystem.Options{KeepDescriptors: true, LargeObjectThreshold: setting.Git.LargeObjectThreshold, AlternatesFS: altFs})
	gogitRepo, err := gogit.Open(storage, fs)
	if err != nil {
		return nil, err
	}

	return &Repository{
		Path:         repoPath,
		gogitRepo:    gogitRepo,
		gogitStorage: storage,
		tagCache:     newObjectCache(),
		Ctx:          ctx,
		objectFormat: ParseGogitHash(plumbing.ZeroHash).Type(),
	}, nil
}

// Close this repository, in particular close the underlying gogitStorage if this is not nil
func (repo *Repository) Close() error {
	if repo == nil || repo.gogitStorage == nil {
		return nil
	}
	if err := repo.gogitStorage.Close(); err != nil {
		gitealog.Error("Error closing storage: %v", err)
	}
	repo.gogitStorage = nil
	repo.LastCommitCache = nil
	repo.tagCache = nil
	return nil
}

// GoGitRepo gets the go-git repo representation
func (repo *Repository) GoGitRepo() *gogit.Repository {
	return repo.gogitRepo
}
