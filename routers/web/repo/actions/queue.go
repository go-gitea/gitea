// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"

	"gitea.dev/modules/util"
	shared_actions "gitea.dev/routers/web/shared/actions"
	"gitea.dev/services/context"
)

// Queue renders this repository's Actions build queue (queued jobs in pickup order plus running jobs)
// inside the Actions tab. The view is read-only: reordering is site-admin only, because queue_rank is a
// global ordering key (see actions_model.MoveQueuedJob).
func Queue(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsActions"] = true
	ctx.Data["PageIsActionsQueue"] = true
	if !ctx.FormBool("refresh") {
		prepareActionsSidebar(ctx)
		if ctx.Written() {
			return
		}
	}
	shared_actions.RenderQueue(ctx, shared_actions.QueueScope{
		RepoID:       ctx.Repo.Repository.ID,
		IsRepo:       true,
		FullTemplate: "repo/actions/queue",
	})
}

// prepareActionsSidebar fills the workflow list for the shared Actions left nav.
func prepareActionsSidebar(ctx *context.Context) {
	commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx, ctx.Repo.Repository.DefaultBranch)
	if errors.Is(err, util.ErrNotExist) {
		return
	} else if err != nil {
		ctx.ServerError("GetBranchCommit", err)
		return
	}

	workflows, _ := prepareWorkflowTemplate(ctx, commit)
	if ctx.Written() {
		return
	}
	scopedNames := prepareScopedWorkflows(ctx, "", 0)
	if ctx.Written() {
		return
	}
	prepareOtherWorkflows(ctx, workflows, scopedNames, "")
}
