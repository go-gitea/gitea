// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
)

const (
	tplSettingsOAuthApplications base.TplName = "user/settings/applications_oauth2_edit"
)

// OAuthApplicationsPost response for adding a oauth2 application
func OAuthApplicationsPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditOAuth2ApplicationForm)
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	if ctx.HasError() {
		loadApplicationsData(ctx)

		ctx.HTML(http.StatusOK, tplSettingsApplications)
		return
	}
	// TODO validate redirect URI
	app, err := auth.CreateOAuth2Application(auth.CreateOAuth2ApplicationOptions{
		Name:         form.Name,
		RedirectURIs: []string{form.RedirectURI},
		UserID:       ctx.Doer.ID,
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
	ctx.HTML(http.StatusOK, tplSettingsOAuthApplications)
}

// OAuthApplicationsEdit response for editing oauth2 application
func OAuthApplicationsEdit(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditOAuth2ApplicationForm)
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	if ctx.HasError() {
		loadApplicationsData(ctx)

		ctx.HTML(http.StatusOK, tplSettingsApplications)
		return
	}
	// TODO validate redirect URI
	var err error
	if ctx.Data["App"], err = auth.UpdateOAuth2Application(auth.UpdateOAuth2ApplicationOptions{
		ID:           ctx.ParamsInt64("id"),
		Name:         form.Name,
		RedirectURIs: []string{form.RedirectURI},
		UserID:       ctx.Doer.ID,
	}); err != nil {
		ctx.ServerError("UpdateOAuth2Application", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.update_oauth2_application_success"))
	ctx.HTML(http.StatusOK, tplSettingsOAuthApplications)
}

// OAuthApplicationsRegenerateSecret handles the post request for regenerating the secret
func OAuthApplicationsRegenerateSecret(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	app, err := auth.GetOAuth2ApplicationByID(ctx.ParamsInt64("id"))
	if err != nil {
		if auth.IsErrOAuthApplicationNotFound(err) {
			ctx.NotFound("Application not found", err)
			return
		}
		ctx.ServerError("GetOAuth2ApplicationByID", err)
		return
	}
	if app.UID != ctx.Doer.ID {
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
	ctx.HTML(http.StatusOK, tplSettingsOAuthApplications)
}

// OAuth2ApplicationShow displays the given application
func OAuth2ApplicationShow(ctx *context.Context) {
	app, err := auth.GetOAuth2ApplicationByID(ctx.ParamsInt64("id"))
	if err != nil {
		if auth.IsErrOAuthApplicationNotFound(err) {
			ctx.NotFound("Application not found", err)
			return
		}
		ctx.ServerError("GetOAuth2ApplicationByID", err)
		return
	}
	if app.UID != ctx.Doer.ID {
		ctx.NotFound("Application not found", nil)
		return
	}
	ctx.Data["App"] = app
	ctx.HTML(http.StatusOK, tplSettingsOAuthApplications)
}

// DeleteOAuth2Application deletes the given oauth2 application
func DeleteOAuth2Application(ctx *context.Context) {
	if err := auth.DeleteOAuth2Application(ctx.FormInt64("id"), ctx.Doer.ID); err != nil {
		ctx.ServerError("DeleteOAuth2Application", err)
		return
	}
	log.Trace("OAuth2 Application deleted: %s", ctx.Doer.Name)

	ctx.Flash.Success(ctx.Tr("settings.remove_oauth2_application_success"))
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/applications",
	})
}

// RevokeOAuth2Grant revokes the grant with the given id
func RevokeOAuth2Grant(ctx *context.Context) {
	if ctx.Doer.ID == 0 || ctx.FormInt64("id") == 0 {
		ctx.ServerError("RevokeOAuth2Grant", fmt.Errorf("user id or grant id is zero"))
		return
	}
	if err := auth.RevokeOAuth2Grant(ctx.FormInt64("id"), ctx.Doer.ID); err != nil {
		ctx.ServerError("RevokeOAuth2Grant", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.revoke_oauth2_grant_success"))
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/applications",
	})
}
