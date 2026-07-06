// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	issues_model "gitea.dev/models/issues"
	"gitea.dev/services/aireview"
	"gitea.dev/services/context"
)

// AIReviewStatus returns the AI review status for a pull request.
func AIReviewStatus(ctx *context.Context) {
	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		ctx.JSON(404, map[string]string{"error": "pull request not found"})
		return
	}

	status, count := aireview.GetReviewStatus(pr.ID)
	ctx.JSON(200, map[string]any{
		"status":       string(status),
		"issue_count":  count,
		"pr_id":        pr.ID,
	})
}
