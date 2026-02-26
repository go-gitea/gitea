// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	shared_mention "code.gitea.io/gitea/routers/web/shared/mention"
	"code.gitea.io/gitea/services/context"
)

// GetMentions returns JSON data for mention autocomplete (assignees, participants, mentionable teams).
func GetMentions(ctx *context.Context) {
	c := shared_mention.NewCollector()

	// Get participants if issue_index is provided
	if issueIndex := ctx.FormInt64("issue_index"); issueIndex > 0 {
		issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, issueIndex)
		if err != nil {
			ctx.NotFoundOrServerError("GetIssueByIndex", issues_model.IsErrIssueNotExist, err)
			return
		}
		userIDs, err := issue.GetParticipantIDsByIssue(ctx)
		if err != nil {
			ctx.ServerError("GetParticipantIDsByIssue", err)
			return
		}
		if len(userIDs) > 0 {
			users, err := user_model.GetUsersByIDs(ctx, userIDs)
			if err != nil {
				ctx.ServerError("GetUsersByIDs", err)
				return
			}
			c.AddUsers(ctx, users)
		}
	}

	// Get repo assignees
	assignees, err := repo_model.GetRepoAssignees(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("GetRepoAssignees", err)
		return
	}
	c.AddUsers(ctx, assignees)

	// Get mentionable teams for org repos
	if err := c.AddMentionableTeams(ctx, ctx.Doer, ctx.Repo.Owner); err != nil {
		ctx.ServerError("AddMentionableTeams", err)
		return
	}

	ctx.JSON(http.StatusOK, c.ResultOrEmpty())
}
