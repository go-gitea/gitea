// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/log"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
)

const (
	tplWatching templates.TplName = "repo/issue/view_content/watching"
)

// IssueWatch sets issue watching
func IssueWatch(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != issue.PosterID && !ctx.Repo.Permission.CanReadIssuesOrPulls(issue.IsPull)) {
		if log.IsTrace() {
			if ctx.IsSigned {
				issueType := "issues"
				if issue.IsPull {
					issueType = "pulls"
				}
				log.Trace("Permission Denied: User %-v not the Poster (ID: %d) and cannot read %s in Repo %-v.\n"+
					"User in Repo has Permissions: %-+v",
					ctx.Doer,
					issue.PosterID,
					issueType,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Not logged in")
			}
		}
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	watch := ctx.FormBool("watch")
	if err := issues_model.CreateOrUpdateIssueWatch(ctx, ctx.Doer.ID, issue.ID, watch); err != nil {
		ctx.ServerError("CreateOrUpdateIssueWatch", err)
		return
	}

	ctx.Data["Issue"] = issue
	ctx.Data["IssueWatch"] = &issues_model.IssueWatch{IsWatching: watch}
	ctx.HTML(http.StatusOK, tplWatching)
}
