// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	shared "code.gitea.io/gitea/routers/web/shared/secrets"
)

func PrepareSecretsData(ctx *context.Context) {
	ctx.Data["DisableSSH"] = setting.SSH.Disabled

	shared.SetSecretsContext(ctx, 0, ctx.Repo.Repository.ID)
	if ctx.Written() {
		return
	}
}

func SecretsPost(ctx *context.Context) {
	shared.PerformSecretsPost(
		ctx,
		0,
		ctx.Repo.Repository.ID,
		ctx.Repo.RepoLink+"/settings/actions",
	)
}

func DeleteSecret(ctx *context.Context) {
	shared.PerformSecretsDelete(
		ctx,
		0,
		ctx.Repo.Repository.ID,
		ctx.Repo.RepoLink+"/settings/actions",
	)
}
