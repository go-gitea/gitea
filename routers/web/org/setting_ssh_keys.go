// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	"code.gitea.io/gitea/modules/templates"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
	mirror_service "code.gitea.io/gitea/services/mirror"
)

const (
	tplSettingsSSHKeys templates.TplName = "org/settings/ssh_keys"
)

// SSHKeys render organization SSH mirror keys page
func SSHKeys(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.settings.ssh_keys")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsSSHKeys"] = true

	if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
		ctx.ServerError("RenderUserOrgHeader", err)
		return
	}

	keypair, err := mirror_service.GetOrCreateSSHKeypairForOrg(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.ServerError("GetOrCreateSSHKeypairForOrg", err)
		return
	}

	ctx.Data["SSHKeypair"] = keypair

	ctx.HTML(http.StatusOK, tplSettingsSSHKeys)
}

// RegenerateSSHKey regenerates the SSH keypair for organization mirror operations
func RegenerateSSHKey(ctx *context.Context) {
	_, err := mirror_service.RegenerateSSHKeypairForOrg(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.ServerError("RegenerateSSHKeypairForOrg", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.mirror_ssh_regenerated"))
	ctx.Redirect(ctx.Org.OrgLink + "/settings/ssh_keys")
}
