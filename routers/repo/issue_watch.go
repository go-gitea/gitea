// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"strconv"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// IssueWatch sets issue watching
func IssueWatch(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != issue.PosterID && !ctx.Repo.CanReadIssuesOrPulls(issue.IsPull)) {
		ctx.Error(403)
		return
	}

	watch, err := strconv.ParseBool(ctx.Req.PostForm.Get("watch"))
	if err != nil {
		ctx.ServerError("watch is not bool", err)
		return
	}

	if err := models.CreateOrUpdateIssueWatch(ctx.User.ID, issue.ID, watch); err != nil {
		ctx.ServerError("CreateOrUpdateIssueWatch", err)
		return
	}

	url := fmt.Sprintf("%s/issues/%d", ctx.Repo.RepoLink, issue.Index)
	ctx.Redirect(url, http.StatusSeeOther)
}
