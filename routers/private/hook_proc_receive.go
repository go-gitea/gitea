// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"errors"
	"net/http"

	issues_model "codenest.dev/models/issues"
	user_model "codenest.dev/models/user"
	"codenest.dev/modules/private"
	"codenest.dev/modules/web"
	"codenest.dev/services/agit"
	codenest_context "codenest.dev/services/context"
)

// HookProcReceive proc-receive hook - only handles agit Proc-Receive requests at present
func HookProcReceive(ctx *codenest_context.PrivateContext) {
	opts := web.GetForm[*private.HookOptions](ctx)
	if !loadContextDoerPermission(ctx, opts.UserID, opts.UserExtDoerData) {
		return
	}

	results, err := agit.ProcReceive(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, &agit.ProcReceiveOptions{
		OldCommitIDs:   opts.OldCommitIDs,
		NewCommitIDs:   opts.NewCommitIDs,
		RefFullNames:   opts.RefFullNames,
		GitPushOptions: opts.GitPushOptions,
		Doer:           ctx.Doer,
	})
	if err != nil {
		if errors.Is(err, issues_model.ErrMustCollaborator) {
			ctx.PrivateUserErrorf(http.StatusUnauthorized, "You must be a collaborator to create pull request.")
		} else if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.PrivateUserErrorf(http.StatusUnauthorized, "Cannot create pull request because you are blocked by the repository owner.")
		} else {
			ctx.PrivateInternalErrorf("agit.ProcReceive failed: %v", err)
		}

		return
	}

	ctx.JSON(http.StatusOK, private.HookProcReceiveResult{
		Results: results,
	})
}
