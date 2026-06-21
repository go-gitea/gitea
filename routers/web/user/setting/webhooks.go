// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"net/http"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	"gitea.dev/models/webhook"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
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

	ws, err := db.Find[webhook.Webhook](ctx, webhook.ListWebhookOptions{OwnerID: ctx.Doer.ID})
	if err != nil {
		ctx.ServerError("ListWebhooksByOpts", err)
		return
	}

	ctx.Data["Webhooks"] = ws
	ctx.HTML(http.StatusOK, tplSettingsHooks)
}

// DeleteWebhook response for delete webhook
func DeleteWebhook(ctx *context.Context) {
	hook, err := webhook.GetWebhookByOwnerID(ctx, ctx.Doer.ID, ctx.FormInt64("id"))
	if err != nil {
		ctx.Flash.Error("GetWebhookByOwnerID: " + err.Error())
		ctx.JSONRedirect(setting.AppSubURL + "/user/settings/hooks")
		return
	}

	if err := webhook.DeleteWebhookByOwnerID(ctx, ctx.Doer.ID, hook.ID); err != nil {
		ctx.Flash.Error("DeleteWebhookByOwnerID: " + err.Error())
	} else {
		audit.Record(ctx, audit_model.UserWebhookRemove, ctx.Doer, ctx.Doer,
			fmt.Sprintf("Removed webhook %s of user %s.", hook.URL, ctx.Doer.Name), "webhook", hook.URL)

		ctx.Flash.Success(ctx.Tr("repo.settings.webhook_deletion_success"))
	}

	ctx.JSONRedirect(setting.AppSubURL + "/user/settings/hooks")
}
