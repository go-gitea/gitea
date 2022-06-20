// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"net/http"
	"net/url"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplRunners    base.TplName = "admin/runner/list"
	tplRunnerNew  base.TplName = "admin/runner/new"
	tplRunnerEdit base.TplName = "admin/runner/edit"
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
	}

	count, err := bots_model.CountRunners(opts)
	if err != nil {
		ctx.ServerError("SearchUsers", err)
		return
	}

	runners, err := bots_model.FindRunners(opts)
	if err != nil {
		ctx.ServerError("SearchUsers", err)
		return
	}
	if err := runners.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	ctx.Data["Runners"] = runners
	ctx.Data["Total"] = count

	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplRunners)
}

// NewRunner render adding a new runner page
func NewRunner(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.runners.new")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRunners"] = true

	ctx.HTML(http.StatusOK, tplRunnerNew)
}

// NewRunnerPost response for adding a new runner
func NewRunnerPost(ctx *context.Context) {
	// form := web.GetForm(ctx).(*forms.AdminCreateRunnerForm)
	ctx.Data["Title"] = ctx.Tr("admin.runners.new")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRunners"] = true

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplRunnerNew)
		return
	}

	// ctx.Flash.Success(ctx.Tr("admin.runners.new_success", u.Name))
	// ctx.Redirect(setting.AppSubURL + "/admin/users/" + strconv.FormatInt(u.ID, 10))
}

// EditRunner show editing runner page
func EditRunner(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.runners.edit")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRunners"] = true

	prepareUserInfo(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplUserEdit)
}

// EditRunnerPost response for editing runner
func EditRunnerPost(ctx *context.Context) {
	// form := web.GetForm(ctx).(*forms.AdminEditRunnerForm)
	ctx.Data["Title"] = ctx.Tr("admin.runners.edit")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRunners"] = true

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplUserEdit)
		return
	}

	ctx.Flash.Success(ctx.Tr("admin.users.update_profile_success"))
	ctx.Redirect(setting.AppSubURL + "/admin/users/" + url.PathEscape(ctx.Params(":userid")))
}

// DeleteRunner response for deleting a runner
func DeleteRunner(ctx *context.Context) {
	ctx.Flash.Success(ctx.Tr("admin.runners.deletion_success"))
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/admin/runners",
	})
}
