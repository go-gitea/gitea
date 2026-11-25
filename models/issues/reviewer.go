// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
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

// reviewerCandidateUserIDs returns collaborator and PR-unit team member IDs that may be
// requested as reviewers, excluding the PR poster. The non-organization owner is not
// included here; callers add that case separately (and without an is_active filter).
func reviewerCandidateUserIDs(ctx context.Context, repo *repo_model.Repository, posterID int64) (container.Set[int64], error) {
	if err := repo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	uniqueUserIDs := make(container.Set[int64])

	collaboratorIDs := make([]int64, 0, 10)
	if err := db.GetEngine(ctx).Table("collaboration").Where("repo_id=?", repo.ID).
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

	uniqueUserIDs.Remove(posterID)
	return uniqueUserIDs, nil
}

// GetReviewers returns users that can be requested to review:
// - Poster should not be listed
// - Collaborators with read access or higher
// - For organization repositories, members of teams with pull-request unit read access or higher
// - Owner is listed if the repository is not owned by an organization, the owner is not the poster, and they are not already included
//
// doerID is kept for call-site compatibility; eligibility does not depend on the doer.
func GetReviewers(ctx context.Context, repo *repo_model.Repository, doerID, posterID int64) ([]*user_model.User, error) {
	_ = doerID

	uniqueUserIDs, err := reviewerCandidateUserIDs(ctx, repo, posterID)
	if err != nil {
		return nil, err
	}

	users := make([]*user_model.User, 0, len(uniqueUserIDs)+1)
	if len(uniqueUserIDs) > 0 {
		if err := db.GetEngine(ctx).In("id", uniqueUserIDs.Values()).
			Where(builder.Eq{"`user`.is_active": true}).
			OrderBy(user_model.GetOrderByName()).
			Find(&users); err != nil {
			return nil, err
		}
	}

	if repo.OwnerID != posterID && !repo.Owner.IsOrganization() && !uniqueUserIDs.Contains(repo.OwnerID) {
		users = append(users, repo.Owner)
	}

	return users, nil
}

// IsUserReviewerCandidate reports whether reviewer is eligible under the same rules as GetReviewers.
func IsUserReviewerCandidate(ctx context.Context, repo *repo_model.Repository, posterID int64, reviewer *user_model.User) (bool, error) {
	if reviewer.ID == posterID {
		return false, nil
	}
	if err := repo.LoadOwner(ctx); err != nil {
		return false, fmt.Errorf("IsUserReviewerCandidate: %w", err)
	}
	if !repo.Owner.IsOrganization() && reviewer.ID == repo.OwnerID {
		return true, nil
	}
	if !reviewer.IsActive {
		return false, nil
	}

	uniqueUserIDs, err := reviewerCandidateUserIDs(ctx, repo, posterID)
	if err != nil {
		return false, fmt.Errorf("IsUserReviewerCandidate: %w", err)
	}
	return uniqueUserIDs.Contains(reviewer.ID), nil
}

// GetReviewerTeams returns teams that can be requested to review (pull-request unit read access or higher).
func GetReviewerTeams(ctx context.Context, repo *repo_model.Repository) ([]*organization.Team, error) {
	if err := repo.LoadOwner(ctx); err != nil {
		return nil, err
	}
	if !repo.Owner.IsOrganization() {
		return nil, nil
	}
	return organization.GetTeamsWithAccessToAnyRepoUnit(ctx, repo.OwnerID, repo.ID, perm.AccessModeRead, unit.TypePullRequests)
}

// IsTeamReviewerCandidate reports whether teamID is eligible under the same rules as GetReviewerTeams.
func IsTeamReviewerCandidate(ctx context.Context, repo *repo_model.Repository, teamID int64) (bool, error) {
	teams, err := GetReviewerTeams(ctx, repo)
	if err != nil {
		return false, fmt.Errorf("IsTeamReviewerCandidate: %w", err)
	}
	return slices.ContainsFunc(teams, func(team *organization.Team) bool {
		return team.ID == teamID
	}), nil
}
