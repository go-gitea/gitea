// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package security

import (
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/openid"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
)

// OpenIDPost response for change user's openid
func OpenIDPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AddOpenIDForm)
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsSecurity"] = true

	if ctx.HasError() {
		loadSecurityData(ctx)

		ctx.HTML(http.StatusOK, tplSettingsSecurity)
		return
	}

	// WARNING: specifying a wrong OpenID here could lock
	// a user out of her account, would be better to
	// verify/confirm the new OpenID before storing it

	// Also, consider allowing for multiple OpenID URIs

	id, err := openid.Normalize(form.Openid)
	if err != nil {
		loadSecurityData(ctx)

		ctx.RenderWithErr(err.Error(), tplSettingsSecurity, &form)
		return
	}
	form.Openid = id
	log.Trace("Normalized id: " + id)

	oids, err := user_model.GetUserOpenIDs(ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetUserOpenIDs", err)
		return
	}
	ctx.Data["OpenIDs"] = oids

	// Check that the OpenID is not already used
	for _, obj := range oids {
		if obj.URI == id {
			loadSecurityData(ctx)

			ctx.RenderWithErr(ctx.Tr("form.openid_been_used", id), tplSettingsSecurity, &form)
			return
		}
	}

	redirectTo := setting.AppURL + "user/settings/security"
	url, err := openid.RedirectURL(id, redirectTo, setting.AppURL)
	if err != nil {
		loadSecurityData(ctx)

		ctx.RenderWithErr(err.Error(), tplSettingsSecurity, &form)
		return
	}
	ctx.Redirect(url)
}

func settingsOpenIDVerify(ctx *context.Context) {
	log.Trace("Incoming call to: " + ctx.Req.URL.String())

	fullURL := setting.AppURL + ctx.Req.URL.String()[1:]
	log.Trace("Full URL: " + fullURL)

	id, err := openid.Verify(fullURL)
	if err != nil {
		ctx.RenderWithErr(err.Error(), tplSettingsSecurity, &forms.AddOpenIDForm{
			Openid: id,
		})
		return
	}

	log.Trace("Verified ID: " + id)

	oid := &user_model.UserOpenID{UID: ctx.Doer.ID, URI: id}
	if err = user_model.AddUserOpenID(ctx, oid); err != nil {
		if user_model.IsErrOpenIDAlreadyUsed(err) {
			ctx.RenderWithErr(ctx.Tr("form.openid_been_used", id), tplSettingsSecurity, &forms.AddOpenIDForm{Openid: id})
			return
		}
		ctx.ServerError("AddUserOpenID", err)
		return
	}
	log.Trace("Associated OpenID %s to user %s", id, ctx.Doer.Name)
	ctx.Flash.Success(ctx.Tr("settings.add_openid_success"))

	ctx.Redirect(setting.AppSubURL + "/user/settings/security")
}

// DeleteOpenID response for delete user's openid
func DeleteOpenID(ctx *context.Context) {
	if err := user_model.DeleteUserOpenID(&user_model.UserOpenID{ID: ctx.FormInt64("id"), UID: ctx.Doer.ID}); err != nil {
		ctx.ServerError("DeleteUserOpenID", err)
		return
	}
	log.Trace("OpenID address deleted: %s", ctx.Doer.Name)

	ctx.Flash.Success(ctx.Tr("settings.openid_deletion_success"))
	ctx.JSON(http.StatusOK, map[string]any{
		"redirect": setting.AppSubURL + "/user/settings/security",
	})
}

// ToggleOpenIDVisibility response for toggle visibility of user's openid
func ToggleOpenIDVisibility(ctx *context.Context) {
	if err := user_model.ToggleUserOpenIDVisibility(ctx.FormInt64("id")); err != nil {
		ctx.ServerError("ToggleUserOpenIDVisibility", err)
		return
	}

	ctx.Redirect(setting.AppSubURL + "/user/settings/security")
}
