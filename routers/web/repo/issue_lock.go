// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

// LockIssue locks an issue. This would limit commenting abilities to
// users with write access to the repo.
func LockIssue(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.IssueLockForm)
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if issue.IsLocked {
		ctx.JSONError(ctx.Tr("repo.issues.lock_duplicate"))
		return
	}

	if err := issues_model.LockIssue(ctx, &issues_model.IssueLockOptions{
		Doer:   ctx.Doer,
		Issue:  issue,
		Reason: form.Reason,
	}); err != nil {
		ctx.ServerError("LockIssue", err)
		return
	}

	ctx.JSONRedirect(issue.Link())
}

// UnlockIssue unlocks a previously locked issue.
func UnlockIssue(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !issue.IsLocked {
		ctx.JSONError(ctx.Tr("repo.issues.unlock_error"))
		return
	}

	if err := issues_model.UnlockIssue(ctx, &issues_model.IssueLockOptions{
		Doer:  ctx.Doer,
		Issue: issue,
	}); err != nil {
		ctx.ServerError("UnlockIssue", err)
		return
	}

	ctx.JSONRedirect(issue.Link())
}
