// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

const (
	// tplAdminHooks template path to render hook settings
	tplAdminHooks base.TplName = "admin/hooks"
)

// DefaultOrSystemWebhooks renders both admin default and system webhook list pages
func DefaultOrSystemWebhooks(ctx *context.Context) {
	var ws []*models.Webhook
	var err error

	// Are we looking at default webhooks?
	if ctx.Params(":configType") == "hooks" {
		ctx.Data["Title"] = ctx.Tr("admin.hooks")
		ctx.Data["Description"] = ctx.Tr("admin.hooks.desc")
		ctx.Data["PageIsAdminHooks"] = true
		ctx.Data["BaseLink"] = setting.AppSubURL + "/admin/hooks"
		ws, err = models.GetDefaultWebhooks()
	} else {
		ctx.Data["Title"] = ctx.Tr("admin.systemhooks")
		ctx.Data["Description"] = ctx.Tr("admin.systemhooks.desc")
		ctx.Data["PageIsAdminSystemHooks"] = true
		ctx.Data["BaseLink"] = setting.AppSubURL + "/admin/system-hooks"
		ws, err = models.GetSystemWebhooks()
	}

	if err != nil {
		ctx.ServerError("GetWebhooksAdmin", err)
		return
	}

	ctx.Data["Webhooks"] = ws
	ctx.HTML(200, tplAdminHooks)
}

// DeleteDefaultOrSystemWebhook handler to delete an admin-defined system or default webhook
func DeleteDefaultOrSystemWebhook(ctx *context.Context) {
	if err := models.DeleteDefaultSystemWebhook(ctx.QueryInt64("id")); err != nil {
		ctx.Flash.Error("DeleteDefaultWebhook: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.webhook_deletion_success"))
	}

	// Are we looking at default webhooks?
	if ctx.Params(":configType") == "hooks" {
		ctx.JSON(200, map[string]interface{}{
			"redirect": setting.AppSubURL + "/admin/hooks",
		})
	} else {
		ctx.JSON(200, map[string]interface{}{
			"redirect": setting.AppSubURL + "/admin/system-hooks",
		})
	}
}
