// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
)

// GetReviewerTeams get all teams can be requested to review
func GetReviewerTeams(repo *repo_model.Repository) ([]*organization.Team, error) {
	if err := repo.GetOwner(db.DefaultContext); err != nil {
		return nil, err
	}
	if !repo.Owner.IsOrganization() {
		return nil, nil
	}

	return organization.GetTeamsWithAccessToRepo(db.DefaultContext, repo.OwnerID, repo.ID, perm.AccessModeRead)
}
