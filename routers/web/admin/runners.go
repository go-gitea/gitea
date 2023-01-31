// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/url"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	actions_shared "code.gitea.io/gitea/routers/web/shared/actions"
)

const (
	tplRunners    base.TplName = "admin/runners/base"
	tplRunnerEdit base.TplName = "admin/runners/edit"
)

// Runners show all the runners
func Runners(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.runners")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRunners"] = true

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	opts := actions_model.FindRunnerOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: 100,
		},
		Sort:   ctx.Req.URL.Query().Get("sort"),
		Filter: ctx.Req.URL.Query().Get("q"),
	}

	actions_shared.RunnersList(ctx, tplRunners, opts)
}

// EditRunner show editing runner page
func EditRunner(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.runners.edit_runner")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRunners"] = true

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	actions_shared.RunnerDetails(ctx, tplRunnerEdit, page, ctx.ParamsInt64(":runnerid"), 0, 0)
}

// EditRunnerPost response for editing runner
func EditRunnerPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.runners.edit")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRunners"] = true
	actions_shared.RunnerDetailsEditPost(ctx, ctx.ParamsInt64(":runnerid"), 0, 0,
		setting.AppSubURL+"/admin/runners/"+url.PathEscape(ctx.Params(":runnerid")))
}

// DeleteRunnerPost response for deleting a runner
func DeleteRunnerPost(ctx *context.Context) {
	actions_shared.RunnerDeletePost(ctx, ctx.ParamsInt64(":runnerid"),
		setting.AppSubURL+"/admin/runners/",
		setting.AppSubURL+"/admin/runners/"+url.PathEscape(ctx.Params(":runnerid")),
	)
}

func ResetRunnerRegistrationToken(ctx *context.Context) {
	actions_shared.RunnerResetRegistrationToken(ctx, 0, 0, setting.AppSubURL+"/admin/runners/")
}
