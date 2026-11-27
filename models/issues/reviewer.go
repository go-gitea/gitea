// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"slices"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"

	"xorm.io/builder"
)

// getReviewerIDs returns collaborators, members of teams with pull request access and the owner of a user's repository, active or not
func getReviewerIDs(ctx context.Context, repo *repo_model.Repository) (container.Set[int64], error) {
	if err := repo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	var userIDs []int64
	if err := db.GetEngine(ctx).Table("collaboration").Where("repo_id=?", repo.ID).
		And("mode >= ?", perm.AccessModeRead).
		Select("user_id").
		Find(&userIDs); err != nil {
		return nil, err
	}

	if repo.Owner.IsOrganization() {
		teamUserIDs, err := organization.GetTeamUserIDsWithAccessToAnyRepoUnit(ctx, repo.OwnerID, repo.ID, perm.AccessModeRead, unit.TypePullRequests)
		if err != nil {
			return nil, err
		}
		userIDs = append(userIDs, teamUserIDs...)
	} else {
		userIDs = append(userIDs, repo.OwnerID)
	}
	return container.SetOf(userIDs...), nil
}

// GetReviewers returns the active users that can be requested to review, except the poster
func GetReviewers(ctx context.Context, repo *repo_model.Repository, posterID int64) ([]*user_model.User, error) {
	userIDs, err := getReviewerIDs(ctx, repo)
	if err != nil {
		return nil, err
	}
	userIDs.Remove(posterID)

	users := make([]*user_model.User, 0, len(userIDs))
	if len(userIDs) > 0 {
		if err := db.GetEngine(ctx).In("id", userIDs.Values()).
			Where(builder.Eq{"`user`.is_active": true}).
			OrderBy(user_model.GetOrderByName()).
			Find(&users); err != nil {
			return nil, err
		}
	}
	return users, nil
}

// IsReviewer reports whether the user is one of GetReviewers, ignoring the poster
func IsReviewer(ctx context.Context, repo *repo_model.Repository, user *user_model.User) (bool, error) {
	userIDs, err := getReviewerIDs(ctx, repo)
	if err != nil {
		return false, err
	}
	return user.IsActive && userIDs.Contains(user.ID), nil
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

// IsReviewerTeam reports whether the team is one of GetReviewerTeams
func IsReviewerTeam(ctx context.Context, repo *repo_model.Repository, team *organization.Team) (bool, error) {
	teams, err := GetReviewerTeams(ctx, repo)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(teams, func(reviewerTeam *organization.Team) bool { return reviewerTeam.ID == team.ID }), nil
}
