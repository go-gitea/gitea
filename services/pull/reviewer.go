// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"

	"xorm.io/builder"
)

// GetReviewers get all users can be requested to review:
// - Poster should not be listed
// - For collaborator, all users that have read access or higher to the repository.
// - For repository under organization, users under the teams which have read permission or higher of pull request unit
// - Owner will be listed if it's not an organization, not the poster and not in the list of reviewers
func GetReviewers(ctx context.Context, repo *repo_model.Repository, doerID, posterID int64) ([]*user_model.User, error) {
	if err := repo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	e := db.GetEngine(ctx)
	uniqueUserIDs := make(container.Set[int64])

	collaboratorIDs := make([]int64, 0, 10)
	if err := e.Table("collaboration").Where("repo_id=?", repo.ID).
		And("mode >= ?", perm.AccessModeRead).
		Select("user_id").
		Find(&collaboratorIDs); err != nil {
		return nil, err
	}
	uniqueUserIDs.AddMultiple(collaboratorIDs...)

	if repo.Owner.IsOrganization() {
		additionalUserIDs := make([]int64, 0, 10)
		if err := e.Table("team_user").
			Join("INNER", "team_repo", "`team_repo`.team_id = `team_user`.team_id").
			Join("INNER", "team_unit", "`team_unit`.team_id = `team_user`.team_id").
			Where("`team_repo`.repo_id = ? AND (`team_unit`.access_mode >= ? AND `team_unit`.`type` = ?)",
				repo.ID, perm.AccessModeRead, unit.TypePullRequests).
			Distinct("`team_user`.uid").
			Select("`team_user`.uid").
			Find(&additionalUserIDs); err != nil {
			return nil, err
		}
		uniqueUserIDs.AddMultiple(additionalUserIDs...)
	}

	uniqueUserIDs.Remove(posterID) // posterID should not be in the list of reviewers

	// Leave a seat for owner itself to append later, but if owner is an organization
	// and just waste 1 unit is cheaper than re-allocate memory once.
	users := make([]*user_model.User, 0, len(uniqueUserIDs)+1)
	if len(uniqueUserIDs) > 0 {
		if err := e.In("id", uniqueUserIDs.Values()).
			Where(builder.Eq{"`user`.is_active": true}).
			OrderBy(user_model.GetOrderByName()).
			Find(&users); err != nil {
			return nil, err
		}
	}

	// add owner after all users are loaded because we can avoid load owner twice
	if repo.OwnerID != posterID && !repo.Owner.IsOrganization() && !uniqueUserIDs.Contains(repo.OwnerID) {
		users = append(users, repo.Owner)
	}

	return users, nil
}

// GetReviewerTeams get all teams can be requested to review
func GetReviewerTeams(ctx context.Context, repo *repo_model.Repository) ([]*organization.Team, error) {
	if err := repo.LoadOwner(ctx); err != nil {
		return nil, err
	}
	if !repo.Owner.IsOrganization() {
		return nil, nil
	}

	return organization.GetTeamsWithAccessToRepoUnit(ctx, repo.OwnerID, repo.ID, perm.AccessModeRead, unit.TypePullRequests)
}
