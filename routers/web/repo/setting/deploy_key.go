// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"

	asymkey_model "gitea.dev/models/asymkey"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	asymkey_service "gitea.dev/services/asymkey"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

// DeployKeys render the deploy-keys list of a repository page
func DeployKeys(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.deploy_keys") + " / " + ctx.Tr("secrets.secrets")
	ctx.Data["PageIsSettingsKeys"] = true
	ctx.Data["DisableSSH"] = setting.SSH.Disabled

	keys, err := db.Find[asymkey_model.DeployKey](ctx, asymkey_model.ListDeployKeysOptions{RepoID: ctx.Repo.Repository.ID})
	if err != nil {
		ctx.ServerError("ListDeployKeys", err)
		return
	}
	ctx.Data["RepoDeployKeys"] = keys

	ctx.HTML(http.StatusOK, tplDeployKeys)
}

// DeployKeysPost response for adding a deploy-key of a repository
func DeployKeysPost(ctx *context.Context) {
	form := context.GetFetchActionForm[*forms.AddKeyForm](ctx)
	if form == nil {
		return
	}
	content, err := asymkey_model.CheckPublicKeyString(form.Content)
	if err != nil {
		if db.IsErrSSHDisabled(err) {
			ctx.JSONError(ctx.Tr("settings.ssh_disabled"))
		} else if asymkey_model.IsErrKeyUnableVerify(err) {
			ctx.JSONErrorWithField(ctx.Tr("form.unable_verify_ssh_key"), "content")
		} else if errors.Is(err, asymkey_model.ErrKeyIsPrivate) {
			ctx.JSONErrorWithField(ctx.Tr("form.must_use_public_key"), "content")
		} else {
			ctx.JSONErrorWithField(ctx.Tr("form.invalid_ssh_key", err.Error()), "content")
		}
		return
	}

	accessMode := util.Iif(form.IsWritable, perm.AccessModeWrite, perm.AccessModeRead)
	key, err := asymkey_model.AddDeployKey(ctx, ctx.Repo.Repository.ID, form.Title, content, accessMode)
	if err != nil {
		switch {
		case asymkey_model.IsErrDeployKeyAlreadyExist(err):
			ctx.JSONErrorWithField(ctx.Tr("repo.settings.key_been_used"), "content")
		case asymkey_model.IsErrKeyAlreadyExist(err):
			ctx.JSONErrorWithField(ctx.Tr("settings.ssh_key_been_used"), "content")
		case asymkey_model.IsErrKeyNameAlreadyUsed(err):
			ctx.JSONErrorWithField(ctx.Tr("repo.settings.key_name_used"), "title")
		case asymkey_model.IsErrDeployKeyNameAlreadyUsed(err):
			ctx.JSONErrorWithField(ctx.Tr("repo.settings.key_name_used"), "title")
		default:
			ctx.ServerError("AddDeployKey", err)
		}
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_key_success", key.Name))
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/keys")
}

// DeleteDeployKey response for deleting a deploy-key
func DeleteDeployKey(ctx *context.Context) {
	if err := asymkey_service.DeleteDeployKey(ctx, ctx.Repo.Repository, ctx.FormInt64("id")); err != nil {
		ctx.ServerError("DeleteDeployKey", err)
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.deploy_key_deletion_success"))
	}
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/keys")
}
