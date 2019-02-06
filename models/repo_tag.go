// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/git"
)

// GetTagsByPath returns repo tags by it's path
func GetTagsByPath(path string) ([]*git.Tag, error) {
	gitRepo, err := git.OpenRepository(path)
	if err != nil {
		return nil, err
	}

	return gitRepo.GetTagInfos()
}

// GetTags return repo's tags
func (repo *Repository) GetTags() ([]*git.Tag, error) {
	return GetTagsByPath(repo.RepoPath())
}
