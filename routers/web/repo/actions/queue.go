// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"

	"gitea.dev/models/unit"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/util"
	shared_actions "gitea.dev/routers/web/shared/actions"
	"gitea.dev/services/context"
)

// Queue renders this repository's Actions build queue (queued jobs in pickup order plus running jobs)
// inside the Actions tab.
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

// prepareActionsSidebar lists the navigation entries without binding run-list filters or validating workflows.
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

	_, entries, err := actions_module.ListWorkflows(ctx, ctx.Repo.GitRepo, commit)
	if err != nil {
		ctx.ServerError("ListWorkflows", err)
		return
	}
	workflows := make([]WorkflowInfo, 0, len(entries))
	for _, entry := range entries {
		workflows = append(workflows, WorkflowInfo{EntryName: entry.Name()})
	}
	ctx.Data["workflows"] = workflows
	ctx.Data["RepoLink"] = ctx.Repo.Repository.Link()
	ctx.Data["ActionsConfig"] = ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions).ActionsConfig()
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
