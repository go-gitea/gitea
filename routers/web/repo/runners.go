// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/url"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/common"
)

const (
	tplRunners    = "repo/settings/runners"
	tplRunnerEdit = "repo/settings/runner_edit"
)

// Runners render runners page
func Runners(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.runners")
	ctx.Data["PageIsSettingsRunners"] = true

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	opts := bots_model.FindRunnerOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: 100,
		},
		Sort:        ctx.Req.URL.Query().Get("sort"),
		Filter:      ctx.Req.URL.Query().Get("q"),
		WithDeleted: false,
		RepoID:      ctx.Repo.Repository.ID,
		OwnerID:     0,
	}

	common.RunnersList(ctx, tplRunners, opts)
}

func RunnersEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.runners")
	ctx.Data["PageIsSettingsRunners"] = true
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	common.RunnerDetails(ctx, tplRunnerEdit, page,
		ctx.ParamsInt64(":runnerid"), 0, ctx.Repo.Repository.ID,
	)
}

func RunnersEditPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.runners")
	ctx.Data["PageIsSettingsRunners"] = true
	common.RunnerDetailsEditPost(ctx, ctx.ParamsInt64(":runnerid"),
		0, ctx.Repo.Repository.ID,
		ctx.Repo.RepoLink+"/settings/runners/"+url.PathEscape(ctx.Params(":runnerid")))
}

func ResetRunnerRegistrationToken(ctx *context.Context) {
	common.RunnerResetRegistrationToken(ctx,
		0, ctx.Repo.Repository.ID,
		ctx.Repo.RepoLink+"/settings/runners")
}

// RunnerDeletePost response for deleting runner
func RunnerDeletePost(ctx *context.Context) {
	common.RunnerDeletePost(ctx, ctx.ParamsInt64(":runnerid"),
		ctx.Repo.RepoLink+"/settings/runners",
		ctx.Repo.RepoLink+"/settings/runners/"+url.PathEscape(ctx.Params(":runnerid")))
}
