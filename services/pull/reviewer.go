// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
)

// GetReviewers get all users can be requested to review.
// Eligibility is defined by issues.GetReviewers / issues.IsUserReviewerCandidate.
func GetReviewers(ctx context.Context, repo *repo_model.Repository, doerID, posterID int64) ([]*user_model.User, error) {
	return issues_model.GetReviewers(ctx, repo, doerID, posterID)
}

// GetReviewerTeams get all teams can be requested to review.
// Eligibility is defined by issues.GetReviewerTeams / issues.IsTeamReviewerCandidate.
func GetReviewerTeams(ctx context.Context, repo *repo_model.Repository) ([]*organization.Team, error) {
	return issues_model.GetReviewerTeams(ctx, repo)
}
