// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/auth/openid"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplSettingsOpenID base.TplName = "user/settings/openid"
)

// SettingsOpenID renders change user's openid page
func SettingsOpenID(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsOpenID"] = true

	if ctx.Query("openid.return_to") != "" {
		settingsOpenIDVerify(ctx)
		return
	}

	openid, err := models.GetUserOpenIDs(ctx.User.ID)
	if err != nil {
		ctx.Handle(500, "GetUserOpenIDs", err)
		return
	}
	ctx.Data["OpenIDs"] = openid

	ctx.HTML(200, tplSettingsOpenID)
}

// SettingsOpenIDPost response for change user's openid
func SettingsOpenIDPost(ctx *context.Context, form auth.AddOpenIDForm) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsOpenID"] = true

	if ctx.HasError() {
		openid, err := models.GetUserOpenIDs(ctx.User.ID)
		if err != nil {
			ctx.Handle(500, "GetUserOpenIDs", err)
			return
		}
		ctx.Data["OpenIDs"] = openid
		ctx.HTML(200, tplSettingsOpenID)
		return
	}

	// WARNING: specifying a wrong OpenID here could lock
	// a user out of her account, would be better to
	// verify/confirm the new OpenID before storing it

	// Also, consider allowing for multiple OpenID URIs

	id, err := openid.Normalize(form.Openid)
	if err != nil {
		ctx.RenderWithErr(err.Error(), tplSettingsOpenID, &form)
		return
	}
	form.Openid = id
	log.Trace("Normalized id: " + id)

	oids, err := models.GetUserOpenIDs(ctx.User.ID)
	if err != nil {
		ctx.Handle(500, "GetUserOpenIDs", err)
		return
	}
	ctx.Data["OpenIDs"] = oids

	// Check that the OpenID is not already used
	for _, obj := range oids {
		if obj.URI == id {
			ctx.RenderWithErr(ctx.Tr("form.openid_been_used", id), tplSettingsOpenID, &form)
			return
		}
	}

	redirectTo := setting.AppURL + "user/settings/openid"
	url, err := openid.RedirectURL(id, redirectTo, setting.AppURL)
	if err != nil {
		ctx.RenderWithErr(err.Error(), tplSettingsOpenID, &form)
		return
	}
	ctx.Redirect(url)
}

func settingsOpenIDVerify(ctx *context.Context) {
	log.Trace("Incoming call to: " + ctx.Req.Request.URL.String())

	fullURL := setting.AppURL + ctx.Req.Request.URL.String()[1:]
	log.Trace("Full URL: " + fullURL)

	oids, err := models.GetUserOpenIDs(ctx.User.ID)
	if err != nil {
		ctx.Handle(500, "GetUserOpenIDs", err)
		return
	}
	ctx.Data["OpenIDs"] = oids

	id, err := openid.Verify(fullURL)
	if err != nil {
		ctx.RenderWithErr(err.Error(), tplSettingsOpenID, &auth.AddOpenIDForm{
			Openid: id,
		})
		return
	}

	log.Trace("Verified ID: " + id)

	oid := &models.UserOpenID{UID: ctx.User.ID, URI: id}
	if err = models.AddUserOpenID(oid); err != nil {
		if models.IsErrOpenIDAlreadyUsed(err) {
			ctx.RenderWithErr(ctx.Tr("form.openid_been_used", id), tplSettingsOpenID, &auth.AddOpenIDForm{Openid: id})
			return
		}
		ctx.Handle(500, "AddUserOpenID", err)
		return
	}
	log.Trace("Associated OpenID %s to user %s", id, ctx.User.Name)
	ctx.Flash.Success(ctx.Tr("settings.add_openid_success"))

	ctx.Redirect(setting.AppSubURL + "/user/settings/openid")
}

// DeleteOpenID response for delete user's openid
func DeleteOpenID(ctx *context.Context) {
	if err := models.DeleteUserOpenID(&models.UserOpenID{ID: ctx.QueryInt64("id"), UID: ctx.User.ID}); err != nil {
		ctx.Handle(500, "DeleteUserOpenID", err)
		return
	}
	log.Trace("OpenID address deleted: %s", ctx.User.Name)

	ctx.Flash.Success(ctx.Tr("settings.openid_deletion_success"))
	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/openid",
	})
}

// ToggleOpenIDVisibility response for toggle visibility of user's openid
func ToggleOpenIDVisibility(ctx *context.Context) {
	if err := models.ToggleUserOpenIDVisibility(ctx.QueryInt64("id")); err != nil {
		ctx.Handle(500, "ToggleUserOpenIDVisibility", err)
		return
	}

	ctx.Redirect(setting.AppSubURL + "/user/settings/openid")
}
