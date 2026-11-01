// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"errors"
	"net/http"

	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/private"
	"gitea.dev/modules/web"
	"gitea.dev/services/agit"
	gitea_context "gitea.dev/services/context"
	"gitea.dev/services/quota"
)

// HookProcReceive proc-receive hook - only handles agit Proc-Receive requests at present
func HookProcReceive(ctx *gitea_context.PrivateContext) {
	opts := web.GetForm[*private.HookOptions](ctx)
	release, err := quota.Bind(ctx, ctx.Repo.Repository, opts.QuotaOperation)
	if err != nil {
		ctx.PrivateInternalErrorf("Resuming quota operation: %v", err)
		return
	}
	defer release()

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
