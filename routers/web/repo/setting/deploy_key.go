// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"

	asymkey_model "gitea.dev/models/asymkey"
	"gitea.dev/models/db"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/web"
	asymkey_service "gitea.dev/services/asymkey"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

// DeployKeys render the deploy keys list of a repository page
func DeployKeys(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.deploy_keys") + " / " + ctx.Tr("secrets.secrets")
	ctx.Data["PageIsSettingsKeys"] = true
	ctx.Data["DisableSSH"] = setting.SSH.Disabled

	keys, err := db.Find[asymkey_model.DeployKey](ctx, asymkey_model.ListDeployKeysOptions{RepoID: ctx.Repo.Repository.ID})
	if err != nil {
		ctx.ServerError("ListDeployKeys", err)
		return
	}
	ctx.Data["Deploykeys"] = keys

	ctx.HTML(http.StatusOK, tplDeployKeys)
}

// DeployKeysPost response for adding a deploy key of a repository
func DeployKeysPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AddKeyForm)

	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	content, err := asymkey_model.CheckPublicKeyString(form.Content)
	if err != nil {
		switch {
		case db.IsErrSSHDisabled(err):
			ctx.JSONError(ctx.Tr("settings.ssh_disabled"))
		case asymkey_model.IsErrKeyUnableVerify(err):
			ctx.JSONError(ctx.Tr("form.unable_verify_ssh_key"))
		case errors.Is(err, asymkey_model.ErrKeyIsPrivate):
			ctx.JSONError(ctx.Tr("form.must_use_public_key"))
		default:
			ctx.JSONError(ctx.Tr("form.invalid_ssh_key", err.Error()))
		}
		return
	}

	key, err := asymkey_model.AddDeployKey(ctx, ctx.Repo.Repository.ID, form.Title, content, !form.IsWritable)
	if err != nil {
		switch {
		case asymkey_model.IsErrDeployKeyAlreadyExist(err):
			ctx.JSONError(ctx.Tr("repo.settings.key_been_used"))
		case asymkey_model.IsErrKeyAlreadyExist(err):
			ctx.JSONError(ctx.Tr("settings.ssh_key_been_used"))
		case asymkey_model.IsErrKeyNameAlreadyUsed(err), asymkey_model.IsErrDeployKeyNameAlreadyUsed(err):
			ctx.JSONError(ctx.Tr("repo.settings.key_name_used"))
		default:
			ctx.ServerError("AddDeployKey", err)
		}
		return
	}

	log.Trace("Deploy key added: %d", ctx.Repo.Repository.ID)
	ctx.Flash.Success(ctx.Tr("repo.settings.add_key_success", key.Name))
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/keys")
}

// DeleteDeployKey response for deleting a deploy key
func DeleteDeployKey(ctx *context.Context) {
	if err := asymkey_service.DeleteDeployKey(ctx, ctx.Repo.Repository, ctx.FormInt64("id")); err != nil {
		ctx.Flash.Error("DeleteDeployKey: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.deploy_key_deletion_success"))
	}

	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/keys")
}
