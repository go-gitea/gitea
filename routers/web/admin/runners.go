// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"net/http"
	"net/url"
	"strings"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
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
		Sort:   ctx.Req.URL.Query().Get("sort"),
		Filter: ctx.Req.URL.Query().Get("q"),
	}

	count, err := bots_model.CountRunners(opts)
	if err != nil {
		ctx.ServerError("AdminRunners", err)
		return
	}

	runners, err := bots_model.FindRunners(opts)
	if err != nil {
		ctx.ServerError("AdminRunners", err)
		return
	}
	if err := runners.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	ctx.Data["Keyword"] = opts.Filter
	ctx.Data["Runners"] = runners
	ctx.Data["Total"] = count

	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplRunners)
}

// EditRunner show editing runner page
func EditRunner(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.runners.edit")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRunners"] = true

	runner, err := bots_model.GetRunnerByID(ctx.ParamsInt64(":runnerid"))
	if err != nil {
		ctx.ServerError("GetRunnerByID", err)
		return
	}
	ctx.Data["Runner"] = runner

	// TODO: get task list for this runner

	ctx.HTML(http.StatusOK, tplRunnerEdit)
}

// EditRunnerPost response for editing runner
func EditRunnerPost(ctx *context.Context) {
	runner, err := bots_model.GetRunnerByID(ctx.ParamsInt64(":runnerid"))
	if err != nil {
		log.Warn("EditRunnerPost.GetRunnerByID failed: %v, url: %s", err, ctx.Req.URL)
		ctx.ServerError("EditRunnerPost.GetRunnerByID", err)
		return
	}

	form := web.GetForm(ctx).(*forms.AdminEditRunnerForm)
	runner.Description = form.Description
	runner.CustomLabels = strings.Split(form.CustomLabels, ",")

	err = bots_model.UpdateRunner(ctx, runner, "description", "custom_labels")
	if err != nil {
		log.Warn("EditRunnerPost.UpdateRunner failed: %v, url: %s", err, ctx.Req.URL)
		ctx.Flash.Warning(ctx.Tr("admin.runners.edit_failed"))
		ctx.Redirect(setting.AppSubURL + "/admin/runners/" + url.PathEscape(ctx.Params(":runnerid")))
		return
	}

	ctx.Data["Title"] = ctx.Tr("admin.runners.edit")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRunners"] = true

	log.Debug("EditRunnerPost success: %s", ctx.Req.URL)

	ctx.Flash.Success(ctx.Tr("admin.runners.edit_success"))
	ctx.Redirect(setting.AppSubURL + "/admin/runners/" + url.PathEscape(ctx.Params(":runnerid")))
}

// DeleteRunner response for deleting a runner
func DeleteRunner(ctx *context.Context) {
	ctx.Flash.Success(ctx.Tr("admin.runners.deletion_success"))
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/admin/runners",
	})
}

/**
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
**/
