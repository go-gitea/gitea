// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	shared "code.gitea.io/gitea/routers/web/shared/secrets"
)

const (
	tplSecrets base.TplName = "repo/settings/secrets"
)

func Secrets(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("secrets.secrets")
	ctx.Data["PageIsSettingsSecrets"] = true
	ctx.Data["DisableSSH"] = setting.SSH.Disabled

	shared.SetSecretsContext(ctx, 0, ctx.Repo.Repository.ID)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplSecrets)
}

func SecretsPost(ctx *context.Context) {
	shared.PerformSecretsPost(
		ctx,
		0,
		ctx.Repo.Repository.ID,
		ctx.Repo.RepoLink+"/settings/secrets",
	)
}

func DeleteSecret(ctx *context.Context) {
	shared.PerformSecretsDelete(
		ctx,
		0,
		ctx.Repo.Repository.ID,
		ctx.Repo.RepoLink+"/settings/secrets",
	)
}
