// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import (
	"errors"
	"path/filepath"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/providers/native"
	"code.gitea.io/gitea/modules/git/service"
	"code.gitea.io/gitea/modules/util"

	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

var gitService (service.GitService) = &GitService{}

func init() {
	git.RegisterService("gogit", gitService)
}

// GitService represents a complete native git service
type GitService struct {
	RepositoryService
	native.ArchiveService
	CommitsInfoService
	native.AttributeService
	native.LogService
	native.IndexService
	native.BlameService
	NoteService
	native.HashService
}

var _ (service.RepositoryService) = RepositoryService{}

// RepositoryService represents the native git RepositoryService
type RepositoryService struct{}

// OpenRepository opens repositories
func (RepositoryService) OpenRepository(path string) (service.Repository, error) {
	repoPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	} else if ok, err := util.IsDir(repoPath); !ok || err != nil {
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
