// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

const (
	// tplAdminHooks template path to render hook settings
	tplAdminHooks base.TplName = "admin/hooks"
)

// renders both admin default and system webhook list pages
func Webhooks(ctx *context.Context) {
	var err error

	ctx.Data["Title"] = ctx.Tr("admin.hooks")
	ctx.Data["PageIsAdminSystemHooks"] = true
	ctx.Data["PageIsAdminDefaultHooks"] = true

	def := make(map[string]any, len(ctx.Data))
	sys := make(map[string]any, len(ctx.Data))
	for k, v := range ctx.Data {
		def[k] = v
		sys[k] = v
	}

	sys["Title"] = ctx.Tr("admin.systemhooks")
	sys["Description"] = ctx.Tr("admin.systemhooks.desc")
	sys["Webhooks"], err = webhook.GetSystemWebhooks(ctx, util.OptionalBoolNone)
	sys["BaseLink"] = setting.AppSubURL + "/admin/hooks/system"
	sys["BaseLinkNew"] = setting.AppSubURL + "/admin/hooks/system"
	if err != nil {
		ctx.ServerError("GetSystemWebhooks", err)
		return
	}

	def["Title"] = ctx.Tr("admin.defaulthooks")
	def["Description"] = ctx.Tr("admin.defaulthooks.desc")
	def["Webhooks"], err = webhook.GetDefaultWebhooks(ctx)
	def["BaseLink"] = setting.AppSubURL + "/admin/hooks/default"
	def["BaseLinkNew"] = setting.AppSubURL + "/admin/hooks/default"
	if err != nil {
		ctx.ServerError("GetDefaultWebhooks", err)
		return
	}

	ctx.Data["DefaultWebhooks"] = def
	ctx.Data["SystemWebhooks"] = sys

	ctx.HTML(http.StatusOK, tplAdminHooks)
}

// handler to delete an admin-defined system or default webhook
func DeleteWebhook(ctx *context.Context) {
	isSystemWebhook := ctx.Params(":configType") == "system"
	if err := webhook.DeleteAdminWebhook(ctx, ctx.FormInt64("id"), isSystemWebhook); err != nil {
		ctx.Flash.Error("DeleteDefaultWebhook: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.webhook_deletion_success"))
	}

	ctx.JSONRedirect(setting.AppSubURL + "/admin/hooks")
}
