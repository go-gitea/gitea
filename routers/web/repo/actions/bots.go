// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"net/http"

	bots_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

const (
	tplListBots  base.TplName = "repo/bots/list"
	tplViewBuild base.TplName = "repo/bots/view"
)

// MustEnableBots check if bots are enabled in settings
func MustEnableBots(ctx *context.Context) {
	if !setting.Bots.Enabled {
		ctx.NotFound("MustEnableBots", nil)
		return
	}

	if unit.TypeBots.UnitGlobalDisabled() {
		ctx.NotFound("MustEnableBots", nil)
		return
	}

	if ctx.Repo.Repository != nil {
		if !ctx.Repo.CanRead(unit.TypeBots) {
			ctx.NotFound("MustEnableBots", nil)
			return
		}
	}
}

func List(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.bots")
	ctx.Data["PageIsBots"] = true

	defaultBranch, err := ctx.Repo.GitRepo.GetDefaultBranch()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	commit, err := ctx.Repo.GitRepo.GetBranchCommit(defaultBranch)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	workflows, err := actions.ListWorkflows(commit)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.Data["workflows"] = workflows
	ctx.Data["RepoLink"] = ctx.Repo.Repository.HTMLURL()

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	workflow := ctx.FormString("workflow")
	ctx.Data["CurWorkflow"] = workflow

	opts := bots_model.FindRunOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
		},
		RepoID:           ctx.Repo.Repository.ID,
		WorkflowFileName: workflow,
	}

	// open counts
	opts.IsClosed = util.OptionalBoolFalse
	numOpenRuns, err := bots_model.CountRuns(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	ctx.Data["NumOpenRuns"] = numOpenRuns

	// closed counts
	opts.IsClosed = util.OptionalBoolTrue
	numClosedRuns, err := bots_model.CountRuns(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	ctx.Data["NumClosedRuns"] = numClosedRuns

	opts.IsClosed = util.OptionalBoolNone
	if ctx.FormString("state") == "closed" {
		opts.IsClosed = util.OptionalBoolTrue
		ctx.Data["IsShowClosed"] = true
	} else {
		opts.IsClosed = util.OptionalBoolFalse
	}
	runs, total, err := bots_model.FindRuns(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	for _, run := range runs {
		run.Repo = ctx.Repo.Repository
	}

	if err := runs.LoadTriggerUser(); err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.Data["Runs"] = runs

	pager := context.NewPagination(int(total), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplListBots)
}
