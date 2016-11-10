// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/user"
)

const (
	SETTINGS_OPTIONS base.TplName = "org/settings/options"
	SETTINGS_DELETE  base.TplName = "org/settings/delete"
	SETTINGS_HOOKS   base.TplName = "org/settings/hooks"
)

func Settings(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.settings")
	ctx.Data["PageIsSettingsOptions"] = true
	ctx.HTML(200, SETTINGS_OPTIONS)
}

func SettingsPost(ctx *context.Context, form auth.UpdateOrgSettingForm) {
	ctx.Data["Title"] = ctx.Tr("org.settings")
	ctx.Data["PageIsSettingsOptions"] = true

	if ctx.HasError() {
		ctx.HTML(200, SETTINGS_OPTIONS)
		return
	}

	org := ctx.Org.Organization

	// Check if organization name has been changed.
	if org.LowerName != strings.ToLower(form.Name) {
		isExist, err := models.IsUserExist(org.ID, form.Name)
		if err != nil {
			ctx.Handle(500, "IsUserExist", err)
			return
		} else if isExist {
			ctx.Data["OrgName"] = true
			ctx.RenderWithErr(ctx.Tr("form.username_been_taken"), SETTINGS_OPTIONS, &form)
			return
		} else if err = models.ChangeUserName(org, form.Name); err != nil {
			if err == models.ErrUserNameIllegal {
				ctx.Data["OrgName"] = true
				ctx.RenderWithErr(ctx.Tr("form.illegal_username"), SETTINGS_OPTIONS, &form)
			} else {
				ctx.Handle(500, "ChangeUserName", err)
			}
			return
		}
		// reset ctx.org.OrgLink with new name
		ctx.Org.OrgLink = setting.AppSubUrl + "/org/" + form.Name
		log.Trace("Organization name changed: %s -> %s", org.Name, form.Name)
	}
	// In case it's just a case change.
	org.Name = form.Name
	org.LowerName = strings.ToLower(form.Name)

	if ctx.User.IsAdmin {
		org.MaxRepoCreation = form.MaxRepoCreation
	}

	org.FullName = form.FullName
	org.Description = form.Description
	org.Website = form.Website
	org.Location = form.Location
	if err := models.UpdateUser(org); err != nil {
		ctx.Handle(500, "UpdateUser", err)
		return
	}
	log.Trace("Organization setting updated: %s", org.Name)
	ctx.Flash.Success(ctx.Tr("org.settings.update_setting_success"))
	ctx.Redirect(ctx.Org.OrgLink + "/settings")
}

func SettingsAvatar(ctx *context.Context, form auth.AvatarForm) {
	form.Source = auth.AvatarLocal
	if err := user.UpdateAvatarSetting(ctx, form, ctx.Org.Organization); err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("org.settings.update_avatar_success"))
	}

	ctx.Redirect(ctx.Org.OrgLink + "/settings")
}

func SettingsDeleteAvatar(ctx *context.Context) {
	if err := ctx.Org.Organization.DeleteAvatar(); err != nil {
		ctx.Flash.Error(err.Error())
	}

	ctx.Redirect(ctx.Org.OrgLink + "/settings")
}

func SettingsDelete(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.settings")
	ctx.Data["PageIsSettingsDelete"] = true

	org := ctx.Org.Organization
	if ctx.Req.Method == "POST" {
		if _, err := models.UserSignIn(ctx.User.Name, ctx.Query("password")); err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_password"), SETTINGS_DELETE, nil)
			} else {
				ctx.Handle(500, "UserSignIn", err)
			}
			return
		}

		if err := models.DeleteOrganization(org); err != nil {
			if models.IsErrUserOwnRepos(err) {
				ctx.Flash.Error(ctx.Tr("form.org_still_own_repo"))
				ctx.Redirect(ctx.Org.OrgLink + "/settings/delete")
			} else {
				ctx.Handle(500, "DeleteOrganization", err)
			}
		} else {
			log.Trace("Organization deleted: %s", org.Name)
			ctx.Redirect(setting.AppSubUrl + "/")
		}
		return
	}

	ctx.HTML(200, SETTINGS_DELETE)
}

func Webhooks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["BaseLink"] = ctx.Org.OrgLink
	ctx.Data["Description"] = ctx.Tr("org.settings.hooks_desc")

	ws, err := models.GetWebhooksByOrgID(ctx.Org.Organization.ID)
	if err != nil {
		ctx.Handle(500, "GetWebhooksByOrgId", err)
		return
	}

	ctx.Data["Webhooks"] = ws
	ctx.HTML(200, SETTINGS_HOOKS)
}

func DeleteWebhook(ctx *context.Context) {
	if err := models.DeleteWebhookByOrgID(ctx.Org.Organization.ID, ctx.QueryInt64("id")); err != nil {
		ctx.Flash.Error("DeleteWebhookByOrgID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.webhook_deletion_success"))
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Org.OrgLink + "/settings/hooks",
	})
}
