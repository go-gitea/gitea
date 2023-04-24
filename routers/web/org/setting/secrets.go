// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"code.gitea.io/gitea/modules/context"
	shared "code.gitea.io/gitea/routers/web/shared/secrets"
)

// Prepare Secrets page under org/settings/actions
func PrepareSecretsData(ctx *context.Context) {
	shared.SetSecretsContext(ctx, ctx.ContextUser.ID, 0)
	if ctx.Written() {
		return
	}
}

// SecretsPost add secrets
func SecretsPost(ctx *context.Context) {
	shared.PerformSecretsPost(
		ctx,
		ctx.ContextUser.ID,
		0,
		ctx.Org.OrgLink+"/settings/actions/secrets",
	)
}

// SecretsDelete delete secrets
func SecretsDelete(ctx *context.Context) {
	shared.PerformSecretsDelete(
		ctx,
		ctx.ContextUser.ID,
		0,
		ctx.Org.OrgLink+"/settings/actions/secrets",
	)
}
