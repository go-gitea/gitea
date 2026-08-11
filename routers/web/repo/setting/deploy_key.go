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
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	asymkey_service "gitea.dev/services/asymkey"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

// DeployKeys render the deploy keys and HTTPS deploy tokens list of a repository page
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

	httpsKeys, err := db.Find[asymkey_model.HTTPSDeployKey](ctx,
		asymkey_model.ListHTTPSDeployKeysOptions{RepoID: ctx.Repo.Repository.ID})
	if err != nil {
		ctx.ServerError("ListHTTPSDeployKeys", err)
		return
	}
	ctx.Data["HTTPSDeploykeys"] = httpsKeys

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

	key, err := asymkey_model.AddDeployKey(ctx, ctx.Repo.Repository.ID, form.Title, content, !form.IsWritable)
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

// DeleteDeployKey response for deleting a deploy key
func DeleteDeployKey(ctx *context.Context) {
	if err := asymkey_service.DeleteDeployKey(ctx, ctx.Repo.Repository, ctx.FormInt64("id")); err != nil {
		ctx.Flash.Error("DeleteDeployKey: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.deploy_key_deletion_success"))
	}

	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/keys")
}

// HTTPSDeployKeysPost handles creation of an HTTPS deploy key for the current
// repository. The plaintext token is rendered inline via ctx.Data so it never
// touches cookie-backed flash storage.
func HTTPSDeployKeysPost(ctx *context.Context) {
	form := web.GetForm[*forms.HTTPSDeployKeyForm](ctx)
	ctx.Data["Title"] = ctx.Tr("repo.settings.deploy_keys")
	ctx.Data["PageIsSettingsKeys"] = true

	if ctx.HasError() {
		ctx.Data["HasError"] = true
		ctx.Data["httpsKeyTitle"] = form.Title
		DeployKeys(ctx)
		ctx.HTML(http.StatusOK, tplDeployKeys)
		return
	}

	key, token, err := asymkey_model.AddHTTPSDeployKey(ctx, ctx.Repo.Repository.ID, form.Title, !form.IsWritable)
	if err != nil {
		switch {
		case asymkey_model.IsErrHTTPSDeployKeyNameAlreadyUsed(err):
			ctx.Data["HasError"] = true
			ctx.Data["Err_Title"] = true
		case errors.Is(err, util.ErrInvalidArgument):
			ctx.Data["HasError"] = true
			ctx.Data["Err_Title"] = true
		default:
			ctx.ServerError("AddHTTPSDeployKey", err)
			return
		}
		ctx.Data["httpsKeyTitle"] = form.Title
		DeployKeys(ctx)
		ctx.HTML(http.StatusOK, tplDeployKeys)
		return
	}

	log.Trace("HTTPS deploy key added: operator=%s repo=%s key=%s (id=%d)",
		ctx.Doer.Name, ctx.Repo.Repository.FullName(), key.Name, key.ID)

	// Render the page inline with the token in ctx.Data.
	// This avoids storing the secret credential in cookie-backed flash.
	DeployKeys(ctx)
	ctx.Data["HTTPSDeployKeyToken"] = token
	ctx.Data["HTTPSDeployKeyName"] = key.Name
	ctx.HTML(http.StatusOK, tplDeployKeys)
}

// DeleteHTTPSDeployKey deletes a single HTTPS deploy key scoped to the
// current repository.
func DeleteHTTPSDeployKey(ctx *context.Context) {
	id := ctx.FormInt64("id")
	if err := asymkey_model.DeleteHTTPSDeployKey(ctx, ctx.Repo.Repository.ID, id); err != nil {
		ctx.Flash.Error("DeleteHTTPSDeployKey: " + err.Error())
	} else {
		log.Trace("HTTPS deploy key deleted: operator=%s repo=%s key-id=%d",
			ctx.Doer.Name, ctx.Repo.Repository.FullName(), id)
		ctx.Flash.Success(ctx.Tr("repo.settings.deploy_key_deletion_success"))
	}

	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/keys")
}
