// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/url"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/context"
	actions_shared "code.gitea.io/gitea/routers/web/shared/actions"
)

const (
	tplRunners    = "repo/settings/runners"
	tplRunnerEdit = "repo/settings/runner_edit"
)

// Runners render runners page
func Runners(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.runners")
	ctx.Data["PageIsSettingsRunners"] = true

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	opts := actions_model.FindRunnerOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: 100,
		},
		Sort:          ctx.Req.URL.Query().Get("sort"),
		Filter:        ctx.Req.URL.Query().Get("q"),
		RepoID:        ctx.Repo.Repository.ID,
		WithAvailable: true,
	}

	actions_shared.RunnersList(ctx, tplRunners, opts)
}

func RunnersEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.runners")
	ctx.Data["PageIsSettingsRunners"] = true
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	actions_shared.RunnerDetails(ctx, tplRunnerEdit, page,
		ctx.ParamsInt64(":runnerid"), 0, ctx.Repo.Repository.ID,
	)
}

func RunnersEditPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.runners")
	ctx.Data["PageIsSettingsRunners"] = true
	actions_shared.RunnerDetailsEditPost(ctx, ctx.ParamsInt64(":runnerid"),
		0, ctx.Repo.Repository.ID,
		ctx.Repo.RepoLink+"/settings/runners/"+url.PathEscape(ctx.Params(":runnerid")))
}

func ResetRunnerRegistrationToken(ctx *context.Context) {
	actions_shared.RunnerResetRegistrationToken(ctx,
		0, ctx.Repo.Repository.ID,
		ctx.Repo.RepoLink+"/settings/runners")
}

// RunnerDeletePost response for deleting runner
func RunnerDeletePost(ctx *context.Context) {
	actions_shared.RunnerDeletePost(ctx, ctx.ParamsInt64(":runnerid"),
		ctx.Repo.RepoLink+"/settings/runners",
		ctx.Repo.RepoLink+"/settings/runners/"+url.PathEscape(ctx.Params(":runnerid")))
}
