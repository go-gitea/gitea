// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// LockIssue locks an issue. This would limit commenting abilities to
// users with write access to the repo.
func LockIssue(ctx *context.Context) {

	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if issue.IsLocked {
		ctx.Flash.Error(ctx.Tr("repo.issues.lock_duplicate"))
		ctx.Redirect(issue.HTMLURL())
		return
	}

	if err := models.LockIssue(ctx.User, issue); err != nil {
		ctx.ServerError("LockIssue", err)
		return
	}

	ctx.Redirect(issue.HTMLURL(), http.StatusSeeOther)
}
