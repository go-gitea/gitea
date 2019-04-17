// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplSettingsOAuthApplications base.TplName = "user/settings/applications_oauth2_edit"
)

// OAuthApplicationsPost response for adding a oauth2 application
func OAuthApplicationsPost(ctx *context.Context, form auth.EditOAuth2ApplicationForm) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	if ctx.HasError() {
		loadApplicationsData(ctx)

		ctx.HTML(200, tplSettingsApplications)
		return
	}
	// TODO validate redirect URI
	app, err := models.CreateOAuth2Application(models.CreateOAuth2ApplicationOptions{
		Name:         form.Name,
		RedirectURIs: []string{form.RedirectURI},
		UserID:       ctx.User.ID,
	})
	if err != nil {
		ctx.ServerError("CreateOAuth2Application", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.create_oauth2_application_success"))
	ctx.Data["App"] = app
	ctx.Data["ClientSecret"], err = app.GenerateClientSecret()
	if err != nil {
		ctx.ServerError("GenerateClientSecret", err)
		return
	}
	ctx.HTML(200, tplSettingsOAuthApplications)
}

// OAuthApplicationsEdit response for editing oauth2 application
func OAuthApplicationsEdit(ctx *context.Context, form auth.EditOAuth2ApplicationForm) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	if ctx.HasError() {
		loadApplicationsData(ctx)

		ctx.HTML(200, tplSettingsApplications)
		return
	}
	// TODO validate redirect URI
	if err := models.UpdateOAuth2Application(models.UpdateOAuth2ApplicationOptions{
		ID:           ctx.ParamsInt64("id"),
		Name:         form.Name,
		RedirectURIs: []string{form.RedirectURI},
		UserID:       ctx.User.ID,
	}); err != nil {
		ctx.ServerError("UpdateOAuth2Application", err)
		return
	}
	var err error
	if ctx.Data["App"], err = models.GetOAuth2ApplicationByID(ctx.ParamsInt64("id")); err != nil {
		ctx.ServerError("GetOAuth2ApplicationByID", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.update_oauth2_application_success"))
	ctx.HTML(200, tplSettingsOAuthApplications)
}

// OAuthApplicationsRegenerateSecret handles the post request for regenerating the secret
func OAuthApplicationsRegenerateSecret(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	app, err := models.GetOAuth2ApplicationByID(ctx.ParamsInt64("id"))
	if err != nil {
		if models.IsErrOAuthApplicationNotFound(err) {
			ctx.NotFound("Application not found", err)
			return
		}
		ctx.ServerError("GetOAuth2ApplicationByID", err)
		return
	}
	if app.UID != ctx.User.ID {
		ctx.NotFound("Application not found", nil)
		return
	}
	ctx.Data["App"] = app
	ctx.Data["ClientSecret"], err = app.GenerateClientSecret()
	if err != nil {
		ctx.ServerError("GenerateClientSecret", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.update_oauth2_application_success"))
	ctx.HTML(200, tplSettingsOAuthApplications)
}

// OAuth2ApplicationShow displays the given application
func OAuth2ApplicationShow(ctx *context.Context) {
	app, err := models.GetOAuth2ApplicationByID(ctx.ParamsInt64("id"))
	if err != nil {
		if models.IsErrOAuthApplicationNotFound(err) {
			ctx.NotFound("Application not found", err)
			return
		}
		ctx.ServerError("GetOAuth2ApplicationByID", err)
		return
	}
	if app.UID != ctx.User.ID {
		ctx.NotFound("Application not found", nil)
		return
	}
	ctx.Data["App"] = app
	ctx.HTML(200, tplSettingsOAuthApplications)
}

// DeleteOAuth2Application deletes the given oauth2 application
func DeleteOAuth2Application(ctx *context.Context) {
	if err := models.DeleteOAuth2Application(ctx.QueryInt64("id"), ctx.User.ID); err != nil {
		ctx.ServerError("DeleteOAuth2Application", err)
		return
	}
	log.Trace("OAuth2 Application deleted: %s", ctx.User.Name)

	ctx.Flash.Success(ctx.Tr("settings.remove_oauth2_application_success"))
	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/applications",
	})
}

// RevokeOAuth2Grant revokes the grant with the given id
func RevokeOAuth2Grant(ctx *context.Context) {
	if ctx.User.ID == 0 || ctx.QueryInt64("id") == 0 {
		ctx.ServerError("RevokeOAuth2Grant", fmt.Errorf("user id or grant id is zero"))
		return
	}
	if err := models.RevokeOAuth2Grant(ctx.QueryInt64("id"), ctx.User.ID); err != nil {
		ctx.ServerError("RevokeOAuth2Grant", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.revoke_oauth2_grant_success"))
	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/applications",
	})
}
