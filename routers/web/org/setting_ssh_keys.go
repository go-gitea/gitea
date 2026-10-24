// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"net/http"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	shared_user "gitea.dev/routers/web/shared/user"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
)

const tplSettingsSSHKeys templates.TplName = "org/settings/ssh_keys"

func SSHKeys(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.settings.ssh_keys")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsSSHKeys"] = true

	if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
		ctx.ServerError("RenderUserOrgHeader", err)
		return
	}

	keypair, err := user_model.GetSSHKeypairByOwner(ctx, ctx.Org.Organization.ID)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		ctx.ServerError("GetSSHKeypairByOwner", err)
		return
	}
	ctx.Data["ManagedSSHKey"] = keypair
	ctx.HTML(http.StatusOK, tplSettingsSSHKeys)
}

func RegenerateSSHKey(ctx *context.Context) {
	keypair, err := user_model.CreateSSHKeypair(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.ServerError("CreateSSHKeypair", err)
		return
	}
	audit.RecordScoped(ctx, ctx.Org.Organization.AsUser(), nil, audit.ManagedSSHKeyGenerate, "fingerprint", keypair.Fingerprint)
	ctx.Flash.Success(ctx.Tr("settings.managed_ssh_generated"))
	ctx.JSONRedirect(ctx.Org.OrgLink + "/settings/ssh_keys")
}
