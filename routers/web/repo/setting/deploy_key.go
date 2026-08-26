// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"

	asymkey_model "gitea.dev/models/asymkey"
	"gitea.dev/models/db"
	deploykey_model "gitea.dev/models/deploykey"
	"gitea.dev/models/perm"
	"gitea.dev/modules/htmlutil"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	asymkey_service "gitea.dev/services/asymkey"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

// DeployKeys render the deploy keys and tokens list of a repository page
func DeployKeys(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.deploy_keys_and_tokens")
	ctx.Data["PageIsSettingsKeys"] = true
	ctx.Data["DisableSSH"] = setting.SSH.Disabled

	keys, err := db.Find[deploykey_model.DeployKey](ctx, deploykey_model.ListDeployKeysOptions{RepoID: ctx.Repo.Repository.ID})
	if err != nil {
		ctx.ServerError("ListDeployKeys", err)
		return
	}
	var sshKeys, tokens []*deploykey_model.DeployKey
	for _, key := range keys {
		if key.KeyType == deploykey_model.KeyTypeToken {
			tokens = append(tokens, key)
		} else {
			sshKeys = append(sshKeys, key)
		}
	}
	ctx.Data["RepoDeployKeys"] = sshKeys
	ctx.Data["DeployTokens"] = tokens

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
	key, err := deploykey_model.AddDeployKeySSH(ctx, ctx.Repo.Repository.ID, form.Title, content, accessMode)
	if err != nil {
		switch {
		case deploykey_model.IsErrDeployKeyAlreadyExist(err):
			ctx.JSONErrorWithField(ctx.Tr("repo.settings.key_been_used"), "content")
		case asymkey_model.IsErrKeyAlreadyExist(err):
			ctx.JSONErrorWithField(ctx.Tr("settings.ssh_key_been_used"), "content")
		case asymkey_model.IsErrKeyNameAlreadyUsed(err), deploykey_model.IsErrDeployKeyNameAlreadyUsed(err):
			ctx.JSONErrorWithField(ctx.Tr("repo.settings.key_name_used"), "title")
		default:
			ctx.ServerError("AddDeployKey", err)
		}
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_key_success", key.Name))
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/keys")
}

// DeleteDeployKey response for deleting a deploy key or a deploy token
func DeleteDeployKey(ctx *context.Context) {
	key, err := asymkey_service.DeleteDeployKey(ctx, ctx.Repo.Repository, ctx.FormInt64("id"))
	if err != nil && !deploykey_model.IsErrDeployKeyNotExist(err) { // a key that is already gone leaves the caller with the state it asked for
		ctx.ServerError("DeleteDeployKey", err)
		return
	}
	if key != nil {
		msg := util.Iif(key.KeyType == deploykey_model.KeyTypeToken, "repo.settings.deploy_token_deletion_success", "repo.settings.deploy_key_deletion_success")
		ctx.Flash.Success(ctx.Tr(msg, key.Name))
	}
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/keys")
}

// DeployTokensPost response for adding a deploy token of a repository
func DeployTokensPost(ctx *context.Context) {
	form := context.GetFetchActionForm[*forms.AddDeployTokenForm](ctx)
	if form == nil {
		return
	}

	key, err := deploykey_model.AddDeployKeyToken(ctx, ctx.Repo.Repository.ID, form.Title, !form.IsWritable)
	if err != nil {
		if deploykey_model.IsErrDeployKeyNameAlreadyUsed(err) {
			ctx.JSONErrorWithField(ctx.Tr("repo.settings.key_name_used"), "title")
		} else {
			ctx.ServerError("AddDeployToken", err)
		}
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.generate_deploy_token_success", htmlutil.HTMLFormat("<code>%s</code>", key.Token)))
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/keys")
}

// RegenerateDeployToken response for replacing the token value of a deploy token
func RegenerateDeployToken(ctx *context.Context) {
	key, err := deploykey_model.RegenerateDeployKeyToken(ctx, ctx.Repo.Repository.ID, ctx.FormInt64("id"))
	if err != nil {
		if deploykey_model.IsErrDeployKeyNotExist(err) {
			ctx.JSONErrorNotFound()
		} else {
			ctx.ServerError("RegenerateDeployToken", err)
		}
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.regenerate_deploy_token_success", htmlutil.HTMLFormat("<code>%s</code>", key.Token)))
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/keys")
}
