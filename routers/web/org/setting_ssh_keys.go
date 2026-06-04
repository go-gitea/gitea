// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	user_model "gitea.dev/models/user"
	ssh_module "gitea.dev/modules/ssh"
	"gitea.dev/modules/templates"
	shared_user "gitea.dev/routers/web/shared/user"
	"gitea.dev/services/context"
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

	keypair, err := ssh_module.GetOrCreateSSHKeypair(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.ServerError("GetOrCreateSSHKeypair", err)
		return
	}

	publicKeyWithComment, _ := keypair.GetPublicKeyWithComment(ctx)
	ctx.Data["SSHKeypair"] = struct {
		*user_model.UserSSHKeypair
		PublicKeyWithComment string
	}{keypair, publicKeyWithComment}

	ctx.HTML(http.StatusOK, tplSettingsSSHKeys)
}

// RegenerateSSHKey regenerates the SSH keypair for organization mirror operations
func RegenerateSSHKey(ctx *context.Context) {
	_, err := user_model.RegenerateUserSSHKeypair(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.ServerError("RegenerateSSHKeypairForOrg", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.managed_ssh_regenerated"))
	ctx.Redirect(ctx.Org.OrgLink + "/settings/ssh_keys")
}
