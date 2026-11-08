// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"net/http"

	"gitea.dev/modules/private"
	"gitea.dev/modules/web"
	"gitea.dev/services/agit"
	gitea_context "gitea.dev/services/context"
)

// HookProcReceive proc-receive hook - only handles agit Proc-Receive requests at present
func HookProcReceive(ctx *gitea_context.PrivateContext) {
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
		ctx.PrivateUserErrorAuto(err)
		return
	}

	ctx.JSON(http.StatusOK, private.HookProcReceiveResult{
		Results: results,
	})
}
