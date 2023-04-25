// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"net/url"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	actions_shared "code.gitea.io/gitea/routers/web/shared/actions"
)

const (
	tplActions    base.TplName = "admin/actions"
	tplRunnerEdit base.TplName = "admin/runners/edit"
)

// Runners render settings/actions/runners page for admin level
func Runners(ctx *context.Context) {
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

	actions_shared.RunnersList(ctx, opts)

	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageType"] = "runners"

	ctx.HTML(http.StatusOK, tplActions)
}

// EditRunner renders runner edit page for admin level
func EditRunner(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.runners.edit_runner")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminActions"] = true

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	actions_shared.RunnerDetails(ctx, page, ctx.ParamsInt64(":runnerid"), 0, 0)
	ctx.HTML(http.StatusOK, tplRunnerEdit)
}

// EditRunnerPost response for editing runner
func EditRunnerPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.runners.edit")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminActions"] = true
	actions_shared.RunnerDetailsEditPost(ctx, ctx.ParamsInt64(":runnerid"), 0, 0,
		setting.AppSubURL+"/admin/actions/runners/"+url.PathEscape(ctx.Params(":runnerid")))
}

// DeleteRunnerPost response for deleting a runner
func DeleteRunnerPost(ctx *context.Context) {
	actions_shared.RunnerDeletePost(ctx, ctx.ParamsInt64(":runnerid"),
		setting.AppSubURL+"/admin/actions/runners",
		setting.AppSubURL+"/admin/actions/runners/"+url.PathEscape(ctx.Params(":runnerid")),
	)
}

func ResetRunnerRegistrationToken(ctx *context.Context) {
	actions_shared.RunnerResetRegistrationToken(ctx, 0, 0, setting.AppSubURL+"/admin/actions/runners")
}

func RedirectToDefaultSetting(ctx *context.Context) {
	ctx.Redirect(setting.AppSubURL + "/admin/actions/runners")
}
