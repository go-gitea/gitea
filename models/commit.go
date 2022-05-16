// Copyright 2021 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
)

// ConvertFromGitCommit converts git commits into SignCommitWithStatuses
func ConvertFromGitCommit(commits []*git.Commit, repo *repo_model.Repository) ([]*SignCommitWithStatuses, error) {
	userCommits, err := asymkey_model.ValidateCommitsWithEmails(commits)
	if err != nil {
		return nil, err
	}
	return ParseCommitsWithStatus(
		asymkey_model.ParseCommitsWithSignature(
			userCommits,
			repo.GetTrustModel(),
			func(user *asymkey_model.User) (bool, error) {
				return IsOwnerMemberCollaborator(repo, user.ID)
			},
		),
		repo,
	), nil
}
