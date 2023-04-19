package setting

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

const (
	tplSettingsActions base.TplName = "user/settings/actions"
)

func Actions(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsSettingsActions"] = true
	GetSecrets(ctx)
	ctx.HTML(http.StatusOK, tplSettingsActions)
}
