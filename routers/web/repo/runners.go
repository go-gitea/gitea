package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/context"
)

// Runners render runners page
func Runners(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.hooks")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["BaseLink"] = ctx.Repo.RepoLink + "/settings/hooks"
	ctx.Data["BaseLinkNew"] = ctx.Repo.RepoLink + "/settings/hooks"
	ctx.Data["Description"] = ctx.Tr("repo.settings.hooks_desc", "https://docs.gitea.io/en-us/webhooks/")

	ws, err := webhook.ListWebhooksByOpts(ctx, &webhook.ListWebhookOptions{RepoID: ctx.Repo.Repository.ID})
	if err != nil {
		ctx.ServerError("GetWebhooksByRepoID", err)
		return
	}
	ctx.Data["Webhooks"] = ws

	ctx.HTML(http.StatusOK, tplHooks)
}
