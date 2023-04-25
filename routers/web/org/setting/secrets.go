// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	shared "code.gitea.io/gitea/routers/web/shared/secrets"
)

// Secrets render settings/actions/secrets page for organization level
func Secrets(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageType"] = "secrets"
	ctx.Data["PageIsOrgSettingsSecrets"] = true
	shared.SetSecretsContext(ctx, ctx.ContextUser.ID, 0)
	if ctx.Written() {
		return
	}
	ctx.HTML(http.StatusOK, tplSettingsActions)
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
