// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
)

// GetActionIssue will return the issue which is used in the context.
func GetActionIssue(ctx *context.Context) *issues_model.Issue {
	issue, err := issues_model.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		ctx.NotFoundOrServerError("GetIssueByIndex", issues_model.IsErrIssueNotExist, err)
		return nil
	}
	issue.Repo = ctx.Repo.Repository
	checkIssueRights(ctx, issue)
	if ctx.Written() {
		return nil
	}
	if err = issue.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", nil)
		return nil
	}
	return issue
}

func checkIssueRights(ctx *context.Context, issue *issues_model.Issue) {
	if issue.IsPull && !ctx.Repo.CanRead(unit.TypePullRequests) ||
		!issue.IsPull && !ctx.Repo.CanRead(unit.TypeIssues) {
		ctx.NotFound("IssueOrPullRequestUnitNotAllowed", nil)
	}
}

// StopTimerIfAvailable stop timer
func StopTimerIfAvailable(user *user_model.User, issue *issues_model.Issue) error {
	if issues_model.StopwatchExists(user.ID, issue.ID) {
		if err := issues_model.CreateOrStopIssueStopwatch(user, issue); err != nil {
			return err
		}
	}

	return nil
}
