// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"github.com/pkg/errors"
)

// IsCommitStatusContextSuccess returns true if all required status check contexts succeed.
func IsCommitStatusContextSuccess(commitStatuses []*models.CommitStatus, requiredContexts []string) bool {
	// If no specific context is required, require that last commit status is a success
	if len(requiredContexts) == 0 {
		status := models.CalcCommitStatus(commitStatuses)
		if status == nil || status.State != models.CommitStatusSuccess {
			return false
		}
		return true
	}

	for _, ctx := range requiredContexts {
		var found bool
		for _, commitStatus := range commitStatuses {
			if commitStatus.Context == ctx {
				if commitStatus.State != models.CommitStatusSuccess {
					return false
				}

				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// IsPullCommitStatusPass returns if all required status checks PASS
func IsPullCommitStatusPass(pr *models.PullRequest) (bool, error) {
	if err := pr.LoadProtectedBranch(); err != nil {
		return false, errors.Wrap(err, "GetLatestCommitStatus")
	}
	if pr.ProtectedBranch == nil || !pr.ProtectedBranch.EnableStatusCheck {
		return true, nil
	}

	// check if all required status checks are successful
	headGitRepo, err := git.OpenRepository(pr.HeadRepo.RepoPath())
	if err != nil {
		return false, errors.Wrap(err, "OpenRepository")
	}
	defer headGitRepo.Close()

	if !headGitRepo.IsBranchExist(pr.HeadBranch) {
		return false, errors.New("Head branch does not exist, can not merge")
	}

	sha, err := headGitRepo.GetBranchCommitID(pr.HeadBranch)
	if err != nil {
		return false, errors.Wrap(err, "GetBranchCommitID")
	}

	if err := pr.LoadBaseRepo(); err != nil {
		return false, errors.Wrap(err, "LoadBaseRepo")
	}

	commitStatuses, err := models.GetLatestCommitStatus(pr.BaseRepo, sha, 0)
	if err != nil {
		return false, errors.Wrap(err, "GetLatestCommitStatus")
	}

	return IsCommitStatusContextSuccess(commitStatuses, pr.ProtectedBranch.StatusCheckContexts), nil
}
