// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
)

// GetBranch returns a branch by its name
func GetBranch(repo *models.Repository, branch string) (*git.Branch, error) {
	if len(branch) == 0 {
		return nil, fmt.Errorf("GetBranch: empty string for branch")
	}
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	return gitRepo.GetBranch(branch)
}
