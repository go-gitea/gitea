// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
)

// CanUserForkBetweenOwners returns true if user can fork between owners.
// By default, a user can fork a repository from another owner, but not from themselves.
// Many users really like to fork their own repositories, so add an experimental setting to allow this.
func CanUserForkBetweenOwners(id1, id2 int64) bool {
	if id1 != id2 {
		return true
	}
	return setting.Repository.AllowForkIntoSameOwner
}

// CanUserForkRepo returns true if specified user can fork repository.
// An owner may already have one or more forks of the repository; that doesn't prevent forking
// again under a different name, so existing forks are not considered here.
func CanUserForkRepo(ctx context.Context, user *user_model.User, repo *repo_model.Repository) (bool, error) {
	if user == nil {
		return false, nil
	}
	if CanUserForkBetweenOwners(repo.OwnerID, user.ID) {
		return true, nil
	}
	ownedOrgs, err := organization.GetOrgsCanCreateRepoByUserID(ctx, user.ID)
	if err != nil {
		return false, err
	}
	for _, org := range ownedOrgs {
		if repo.OwnerID != org.ID {
			return true, nil
		}
	}
	return false, nil
}
