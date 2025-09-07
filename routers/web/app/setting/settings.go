package setting

import (
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
)

const (
	tplViewProfile  templates.TplName = "apps/settings/profile"
	tplViewSecurity templates.TplName = "apps/settings/security"
)

func Profile(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("app.settings")

	ctx.Data["PageIsAppSettings"] = true
	ctx.Data["PageIsSettingsProfile"] = true

	ctx.HTML(200, tplViewProfile)
}

func Security(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("app.settings.security")

	ctx.Data["PageIsAppSettings"] = true
	ctx.Data["PageIsSettingsSecurity"] = true

	ctx.HTML(200, tplViewSecurity)
}
