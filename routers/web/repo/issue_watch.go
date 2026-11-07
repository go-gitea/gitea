// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
)

const (
	tplWatching templates.TplName = "repo/issue/view_content/watching"
)

// IssueWatch sets issue watching
func IssueWatch(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if issue == nil {
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
