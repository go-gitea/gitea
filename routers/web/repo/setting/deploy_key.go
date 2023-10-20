// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/forms"
)

// DeployKeys render the deploy keys list of a repository page
func DeployKeys(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.deploy_keys") + " / " + ctx.Tr("secrets.secrets")
	ctx.Data["PageIsSettingsKeys"] = true
	ctx.Data["DisableSSH"] = setting.SSH.Disabled

	keys, err := asymkey_model.ListDeployKeys(ctx, &asymkey_model.ListDeployKeysOptions{RepoID: ctx.Repo.Repository.ID})
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
	ctx.Data["Title"] = ctx.Tr("repo.settings.deploy_keys")
	ctx.Data["PageIsSettingsKeys"] = true
	ctx.Data["DisableSSH"] = setting.SSH.Disabled

	keys, err := asymkey_model.ListDeployKeys(ctx, &asymkey_model.ListDeployKeysOptions{RepoID: ctx.Repo.Repository.ID})
	if err != nil {
		ctx.ServerError("ListDeployKeys", err)
		return
	}
	ctx.Data["Deploykeys"] = keys

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplDeployKeys)
		return
	}

	content, err := asymkey_model.CheckPublicKeyString(form.Content)
	if err != nil {
		if db.IsErrSSHDisabled(err) {
			ctx.Flash.Info(ctx.Tr("settings.ssh_disabled"))
		} else if asymkey_model.IsErrKeyUnableVerify(err) {
			ctx.Flash.Info(ctx.Tr("form.unable_verify_ssh_key"))
		} else if err == asymkey_model.ErrKeyIsPrivate {
			ctx.Data["HasError"] = true
			ctx.Data["Err_Content"] = true
			ctx.Flash.Error(ctx.Tr("form.must_use_public_key"))
		} else {
			ctx.Data["HasError"] = true
			ctx.Data["Err_Content"] = true
			ctx.Flash.Error(ctx.Tr("form.invalid_ssh_key", err.Error()))
		}
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/keys")
		return
	}

	key, err := asymkey_model.AddDeployKey(ctx, ctx.Repo.Repository.ID, form.Title, content, !form.IsWritable)
	if err != nil {
		ctx.Data["HasError"] = true
		switch {
		case asymkey_model.IsErrDeployKeyAlreadyExist(err):
			ctx.Data["Err_Content"] = true
			ctx.RenderWithErr(ctx.Tr("repo.settings.key_been_used"), tplDeployKeys, &form)
		case asymkey_model.IsErrKeyAlreadyExist(err):
			ctx.Data["Err_Content"] = true
			ctx.RenderWithErr(ctx.Tr("settings.ssh_key_been_used"), tplDeployKeys, &form)
		case asymkey_model.IsErrKeyNameAlreadyUsed(err):
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErr(ctx.Tr("repo.settings.key_name_used"), tplDeployKeys, &form)
		case asymkey_model.IsErrDeployKeyNameAlreadyUsed(err):
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErr(ctx.Tr("repo.settings.key_name_used"), tplDeployKeys, &form)
		default:
			ctx.ServerError("AddDeployKey", err)
		}
		return
	}

	log.Trace("Deploy key added: %d", ctx.Repo.Repository.ID)
	ctx.Flash.Success(ctx.Tr("repo.settings.add_key_success", key.Name))
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/keys")
}

// DeleteDeployKey response for deleting a deploy key
func DeleteDeployKey(ctx *context.Context) {
	if err := asymkey_service.DeleteDeployKey(ctx, ctx.Doer, ctx.FormInt64("id")); err != nil {
		ctx.Flash.Error("DeleteDeployKey: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.deploy_key_deletion_success"))
	}

	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/keys")
}
