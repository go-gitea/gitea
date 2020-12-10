// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"errors"
	"path/filepath"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
	"code.gitea.io/gitea/modules/util"
)

var gitService (service.GitService) = &GitService{}

func init() {
	git.RegisterService("native", gitService)
	// Set the native service as the default
	git.RegisterService("", gitService)
}

// GitService represents a complete native git service
type GitService struct {
	RepositoryService
	ArchiveService
	CommitsInfoService
	AttributeService
	LogService
	IndexService
	BlameService
	NoteService
	HashService
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
		if err == nil {
			return nil, errors.New("no such file or directory")
		}
		return nil, err
	}
	return &Repository{
		path: repoPath,
	}, nil
}
