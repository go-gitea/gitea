// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strconv"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

// IssueWatch sets issue watching
func IssueWatch(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != issue.PosterID && !ctx.Repo.CanReadIssuesOrPulls(issue.IsPull)) {
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
		ctx.Error(http.StatusForbidden)
		return
	}

	watch, err := strconv.ParseBool(ctx.Req.PostForm.Get("watch"))
	if err != nil {
		ctx.ServerError("watch is not bool", err)
		return
	}

	if err := issues_model.CreateOrUpdateIssueWatch(ctx, ctx.Doer.ID, issue.ID, watch); err != nil {
		ctx.ServerError("CreateOrUpdateIssueWatch", err)
		return
	}

	ctx.Redirect(issue.Link())
}
