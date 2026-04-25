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
// - Users with visibility the doer cannot see are filtered out via BuildCanSeeUserCondition
func GetReviewers(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, posterID int64) ([]*user_model.User, error) {
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
		additionalUserIDs, err := organization.GetTeamUserIDsWithAccessToAnyRepoUnit(ctx, repo.OwnerID, repo.ID, perm.AccessModeRead, unit.TypePullRequests)
		if err != nil {
			return nil, err
		}
		uniqueUserIDs.AddMultiple(additionalUserIDs...)
	}

	uniqueUserIDs.Remove(posterID) // posterID should not be in the list of reviewers

	// Leave a seat for owner itself to append later, but if owner is an organization
	// and just waste 1 unit is cheaper than re-allocate memory once.
	users := make([]*user_model.User, 0, len(uniqueUserIDs)+1)
	if len(uniqueUserIDs) > 0 {
		cond := builder.And(
			builder.In("`user`.id", uniqueUserIDs.Values()),
			builder.Eq{"`user`.is_active": true},
		)
		// Hide users the doer cannot see (visibility=private when not in same org/self).
		// BuildCanSeeUserCondition returns nil for admins (no extra filter).
		// Regression guard for issue #37371.
		if visCond := user_model.BuildCanSeeUserCondition(doer); visCond != nil {
			cond = cond.And(visCond)
		}
		if err := e.Where(cond).
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

	return organization.GetTeamsWithAccessToAnyRepoUnit(ctx, repo.OwnerID, repo.ID, perm.AccessModeRead, unit.TypePullRequests)
}
