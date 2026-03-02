// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
)

const (
	tplSettingsHooks templates.TplName = "user/settings/hooks"
)

// Webhooks render webhook list page
func Webhooks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings_title")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["BaseLink"] = setting.AppSubURL + "/user/settings/hooks"
	ctx.Data["BaseLinkNew"] = setting.AppSubURL + "/user/settings/hooks"
	ctx.Data["Description"] = ctx.Tr("settings.hooks.desc")
	ctx.Data["UserDisabledFeatures"] = user_model.DisabledFeaturesWithLoginType(ctx.Doer)

	// Get current page from the URL
	page := max(ctx.FormInt("page"), 1)

	// Initialize options using the logged-in User's ID (ctx.Doer.ID)
	opts := webhook.ListWebhookOptions{
		OwnerID: ctx.Doer.ID,
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: setting.UI.Admin.UserPagingNum,
		},
	}

	// Use FindAndCount to get the paginated list and total count
	ws, count, err := db.FindAndCount[webhook.Webhook](ctx, opts)
	if err != nil {
		ctx.ServerError("ListWebhooksByOpts", err)
		return
	}

	// Set up the Pager for the template
	ctx.Data["Page"] = context.NewPagination(int(count), opts.PageSize, page, 5)
	ctx.Data["Webhooks"] = ws
	ctx.HTML(http.StatusOK, tplSettingsHooks)
}

// DeleteWebhook response for delete webhook
func DeleteWebhook(ctx *context.Context) {
	if err := webhook.DeleteWebhookByOwnerID(ctx, ctx.Doer.ID, ctx.FormInt64("id")); err != nil {
		ctx.Flash.Error("DeleteWebhookByOwnerID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.webhook_deletion_success"))
	}

	ctx.JSONRedirect(setting.AppSubURL + "/user/settings/hooks")
}
