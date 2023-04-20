// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	shared "code.gitea.io/gitea/routers/web/shared/secrets"
)

func prepareSecretsData(ctx *context.Context) {
	shared.SetSecretsContext(ctx, ctx.Doer.ID, 0)
	if ctx.Written() {
		return
	}
}

func SecretsPost(ctx *context.Context) {
	shared.PerformSecretsPost(
		ctx,
		ctx.Doer.ID,
		0,
		setting.AppSubURL+"/user/settings/actions",
	)
}

func SecretsDelete(ctx *context.Context) {
	shared.PerformSecretsDelete(
		ctx,
		ctx.Doer.ID,
		0,
		setting.AppSubURL+"/user/settings/actions",
	)
}
