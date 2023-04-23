// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	shared "code.gitea.io/gitea/routers/web/shared/secrets"
)

const (
	tplSettingsSecrets base.TplName = "user/settings/secrets"
)

func Secrets(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("secrets.secrets")
	ctx.Data["PageIsSettingsSecrets"] = true

	shared.SetSecretsContext(ctx, ctx.Doer.ID, 0)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplSettingsSecrets)
}

func SecretsPost(ctx *context.Context) {
	shared.PerformSecretsPost(
		ctx,
		ctx.Doer.ID,
		0,
		setting.AppSubURL+"/user/settings/secrets",
	)
}

func SecretsDelete(ctx *context.Context) {
	shared.PerformSecretsDelete(
		ctx,
		ctx.Doer.ID,
		0,
		setting.AppSubURL+"/user/settings/secrets",
	)
}
