// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"net/url"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/common"
)

const (
	tplRunners    base.TplName = "admin/runners/base"
	tplRunnerNew  base.TplName = "admin/runners/new"
	tplRunnerEdit base.TplName = "admin/runners/edit"
)

// Runners show all the runners
func Runners(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.runners")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRunners"] = true

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
		RepoID:      0,
		OwnerID:     0,
	}

	common.RunnersList(ctx, tplRunners, opts)
}

// EditRunner show editing runner page
func EditRunner(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.runners.edit_runner")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRunners"] = true

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	common.RunnerDetails(ctx, tplRunnerEdit, page, ctx.ParamsInt64(":runnerid"), 0, 0)
}

// EditRunnerPost response for editing runner
func EditRunnerPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.runners.edit")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRunners"] = true
	common.RunnerDetailsEditPost(ctx, ctx.ParamsInt64(":runnerid"), 0, 0,
		setting.AppSubURL+"/admin/runners/"+url.PathEscape(ctx.Params(":runnerid")))
}

// DeleteRunnerPost response for deleting a runner
func DeleteRunnerPost(ctx *context.Context) {
	common.RunnerDeletePost(ctx, ctx.ParamsInt64(":runnerid"),
		setting.AppSubURL+"/admin/runners/",
		setting.AppSubURL+"/admin/runners/"+url.PathEscape(ctx.Params(":runnerid")),
	)
}

func ResetRunnerRegistrationToken(ctx *context.Context) {
	common.RunnerResetRegistrationToken(ctx, 0, 0, setting.AppSubURL+"/admin/runners/")
}
