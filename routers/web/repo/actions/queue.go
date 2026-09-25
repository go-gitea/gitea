// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"

	"gitea.dev/modules/util"
	shared_actions "gitea.dev/routers/web/shared/actions"
	"gitea.dev/services/context"
)

// Queue renders this repository's Actions build queue.
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
	shared_actions.RenderQueue(ctx, ctx.Repo.Repository.ID, "repo/actions/queue")
}

// prepareActionsSidebar lists the workflows without binding the runs list filters.
func prepareActionsSidebar(ctx *context.Context) {
	commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx, ctx.Repo.Repository.DefaultBranch)
	if errors.Is(err, util.ErrNotExist) {
		ctx.Data["NotFoundPrompt"] = ctx.Tr("repo.branch.default_branch_not_exist", ctx.Repo.Repository.DefaultBranch)
		ctx.NotFound(nil)
		return
	} else if err != nil {
		ctx.ServerError("GetBranchCommit", err)
		return
	}

	workflows, _ := prepareWorkflowTemplate(ctx, commit)
	if ctx.Written() {
		return
	}
	ctx.Data["CurWorkflow"] = ""
	ctx.Data["CurWorkflowScopedRepoID"] = int64(0)
	ctx.Data["CurActor"] = int64(0)
	ctx.Data["CurStatus"] = 0
	ctx.Data["CurBranch"] = ""

	scopedNames := prepareScopedWorkflows(ctx, "", 0)
	if ctx.Written() {
		return
	}
	prepareOtherWorkflows(ctx, workflows, scopedNames, "")
}
