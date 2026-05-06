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

// DeployKeysPost response for adding a deploy key of a repository
func DeployKeysPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AddKeyForm)
	ctx.Data["Title"] = ctx.Tr("repo.settings.deploy_keys")
	ctx.Data["PageIsSettingsKeys"] = true
	ctx.Data["DisableSSH"] = setting.SSH.Disabled

	keys, err := db.Find[asymkey_model.DeployKey](ctx, asymkey_model.ListDeployKeysOptions{RepoID: ctx.Repo.Repository.ID})
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
			ctx.RenderWithErrDeprecated(ctx.Tr("repo.settings.key_been_used"), tplDeployKeys, &form)
		case asymkey_model.IsErrKeyAlreadyExist(err):
			ctx.Data["Err_Content"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("settings.ssh_key_been_used"), tplDeployKeys, &form)
		case asymkey_model.IsErrKeyNameAlreadyUsed(err):
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("repo.settings.key_name_used"), tplDeployKeys, &form)
		case asymkey_model.IsErrDeployKeyNameAlreadyUsed(err):
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("repo.settings.key_name_used"), tplDeployKeys, &form)
		default:
			ctx.ServerError("AddDeployKey", err)
		}
		return
	}

	log.Trace("Deploy key added: operator=%s repo=%s key=%s (id=%d)",
		ctx.Doer.Name, ctx.Repo.Repository.FullName(), key.Name, key.ID)
	ctx.Flash.Success(ctx.Tr("repo.settings.add_key_success", key.Name))
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/keys")
}

// DeleteDeployKey response for deleting a deploy key
func DeleteDeployKey(ctx *context.Context) {
	id := ctx.FormInt64("id")
	if err := asymkey_service.DeleteDeployKey(ctx, ctx.Repo.Repository, id); err != nil {
		ctx.Flash.Error("DeleteDeployKey: " + err.Error())
	} else {
		log.Trace("Deploy key deleted: operator=%s repo=%s key-id=%d",
			ctx.Doer.Name, ctx.Repo.Repository.FullName(), id)
		ctx.Flash.Success(ctx.Tr("repo.settings.deploy_key_deletion_success"))
	}

	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/keys")
}

// HTTPSDeployKeysPost handles creation of an HTTPS deploy key for the current
// repository. The plaintext token is rendered inline via ctx.Data so it never
// touches cookie-backed flash storage.
func HTTPSDeployKeysPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.HTTPSDeployKeyForm)
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
