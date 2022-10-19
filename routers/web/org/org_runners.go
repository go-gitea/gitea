package org

import (
	"net/http"

	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/context"
)

// Runners render runners page
func Runners(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.settings")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsRunners"] = true
	ctx.Data["BaseLink"] = ctx.Org.OrgLink + "/settings/runners"
	ctx.Data["Description"] = ctx.Tr("org.settings.runners_desc")

	ws, err := webhook.ListWebhooksByOpts(ctx, &webhook.ListWebhookOptions{OrgID: ctx.Org.Organization.ID})
	if err != nil {
		ctx.ServerError("GetWebhooksByOrgId", err)
		return
	}

	ctx.Data["Webhooks"] = ws
	ctx.HTML(http.StatusOK, tplSettingsRunners)
}
