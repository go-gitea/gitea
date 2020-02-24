// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
)

// CountDivergingCommits determines how many commits a branch is ahead or behind the repository's base branch
func CountDivergingCommits(repo *models.Repository, branch string) (*git.DivergeObject, error) {
	divergence, err := git.GetDivergingCommits(repo.RepoPath(), repo.DefaultBranch, branch)
	if err != nil {
		return nil, err
	}
	return &divergence, nil
}
