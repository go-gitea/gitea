// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	shared "code.gitea.io/gitea/routers/web/shared/secrets"
)

const (
	tplSettingsSecrets base.TplName = "org/settings/secrets"
)

// Secrets render organization secrets page
func Secrets(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("secrets.secrets")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsSecrets"] = true

	shared.SetSecretsContext(ctx, ctx.ContextUser.ID, 0)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplSettingsSecrets)
}

// SecretsPost add secrets
func SecretsPost(ctx *context.Context) {
	shared.PerformSecretsPost(
		ctx,
		ctx.ContextUser.ID,
		0,
		ctx.Org.OrgLink+"/settings/secrets",
	)
}

// SecretsDelete delete secrets
func SecretsDelete(ctx *context.Context) {
	shared.PerformSecretsDelete(
		ctx,
		ctx.ContextUser.ID,
		0,
		ctx.Org.OrgLink+"/settings/secrets",
	)
}
