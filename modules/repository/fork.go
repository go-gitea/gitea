// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
)

// CanUserForkRepo returns true if specified user can fork repository.
func CanUserForkRepo(user *user_model.User, repo *repo_model.Repository) (bool, error) {
	if user == nil {
		return false, nil
	}
	if repo.OwnerID != user.ID && !repo_model.HasForkedRepo(user.ID, repo.ID) {
		return true, nil
	}
	ownedOrgs, err := organization.GetOrgsCanCreateRepoByUserID(user.ID)
	if err != nil {
		return false, err
	}
	for _, org := range ownedOrgs {
		if repo.OwnerID != org.ID && !repo_model.HasForkedRepo(org.ID, repo.ID) {
			return true, nil
		}
	}
	return false, nil
}
