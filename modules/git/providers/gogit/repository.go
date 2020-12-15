// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import (
	"errors"
	"fmt"
	"path/filepath"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/service"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// GetGoGitRepo gets the go-git repository for this repository
// if the repository is not ours we will return nil and an error
func GetGoGitRepo(repo service.Repository) (*gogit.Repository, error) {
	r, ok := repo.(*Repository)
	if !ok {
		return nil, fmt.Errorf("Not a go git repository")
	}

	return r.gogitRepo, nil
}

var _ (service.Repository) = &Repository{}

// Repository represents a git repository
type Repository struct {
	path string

	gogitRepo    *gogit.Repository
	gogitStorage *filesystem.Storage

	gpgSettings *service.GPGSettings
}

// OpenRepository opens the repository at the given path.
func OpenRepository(repoPath string) (*Repository, error) {
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	} else if ok, err := util.IsDir(repoPath); err != nil {
		return nil, err
	} else if !ok {
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
	storage := filesystem.NewStorageWithOptions(fs, cache.NewObjectLRUDefault(), filesystem.Options{KeepDescriptors: true})
	gogitRepo, err := gogit.Open(storage, fs)
	if err != nil {
		return nil, err
	}

	return &Repository{
		path:         repoPath,
		gogitRepo:    gogitRepo,
		gogitStorage: storage,
	}, nil
}

// Close this repository, in particular close the underlying gogitStorage if this is not nil
func (repo *Repository) Close() error {
	if repo == nil || repo.gogitStorage == nil {
		return nil
	}
	if err := repo.gogitStorage.Close(); err != nil {
		log.Error("Error closing storage: %v", err)
		return err
	}
	return nil
}

// Path is the filesystem path for the repository
func (repo *Repository) Path() string {
	return repo.path
}

//  _
// |_) |  _  |_
// |_) | (_) |_)
//

// GetBlob finds the blob object in the repository.
func (repo *Repository) GetBlob(idStr string) (service.Blob, error) {
	id, err := SHA1{}.FromString(idStr)
	if err != nil {
		return nil, err
	}

	return &Blob{Object: &Object{hash: id, repo: repo}}, nil
}

//  __  _   __
// /__ |_) /__
// \_| |   \_|
//

// GetDefaultPublicGPGKey will return and cache the default public GPG settings for this repository
func (repo *Repository) GetDefaultPublicGPGKey(forceUpdate bool) (*service.GPGSettings, error) {
	if repo.gpgSettings != nil && !forceUpdate {
		return repo.gpgSettings, nil
	}

	var err error
	repo.gpgSettings, err = common.GetDefaultPublicGPGKey(repo.Path())
	return repo.gpgSettings, err
}

//  _
// |_) |  _. ._ _   _
// |_) | (_| | | | (/_
//

// LineBlame returns the latest commit at the given line
func (repo *Repository) LineBlame(revision, path, file string, line uint) (service.Commit, error) {
	res, err := git.NewCommand("blame", fmt.Sprintf("-L %d,%d", line, line), "-p", revision, "--", file).RunInDir(path)
	if err != nil {
		return nil, err
	}
	if len(res) < 40 {
		return nil, fmt.Errorf("invalid result of blame: %s", res)
	}
	return repo.GetCommit(res[:40])
}

//  __
// (_   _  ._    o  _  _
// 	__) (/_ |  \/ | (_ (/_
//

// Service returns this repositories preferred service
func (repo *Repository) Service() service.GitService {
	return gitService
}
