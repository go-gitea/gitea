// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package admin

package admin

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

var (
	// tplSettingsLabels template path for render application settings
	tplSettingsApplications base.TplName = "admin/applications/list"
	// tplSettingsLabels template path for render application edit settings
	tplSettingsEditApplication base.TplName = "admin/applications/edit"
)

const instanceOwnerUserID = 0

// Applications render admin applications page
func Applications(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.applications")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminApplications"] = true

	apps, err := auth.GetOAuth2ApplicationsByUserID(ctx, instanceOwnerUserID)
	if err != nil {
		ctx.ServerError("GetOAuth2ApplicationsByUserID", err)
		return
	}
	ctx.Data["Applications"] = apps

	ctx.HTML(http.StatusOK, tplSettingsApplications)
}

// ApplicationsPost response for adding an oauth2 application
func ApplicationsPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditOAuth2ApplicationForm)
	ctx.Data["Title"] = ctx.Tr("settings.applications")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminApplications"] = true

	if ctx.HasError() {
		apps, err := auth.GetOAuth2ApplicationsByUserID(ctx, instanceOwnerUserID)
		if err != nil {
			ctx.ServerError("GetOAuth2ApplicationsByUserID", err)
			return
		}
		ctx.Data["Applications"] = apps

		ctx.HTML(http.StatusOK, tplSettingsApplications)
		return
	}

	app, err := auth.CreateOAuth2Application(ctx, auth.CreateOAuth2ApplicationOptions{
		Name:         form.Name,
		RedirectURIs: []string{form.RedirectURI},
		UserID:       instanceOwnerUserID,
	})
	if err != nil {
		ctx.ServerError("CreateOAuth2Application", err)
		return
	}
	ctx.Data["App"] = app
	ctx.Data["ClientSecret"], err = app.GenerateClientSecret()
	if err != nil {
		ctx.ServerError("GenerateClientSecret", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.create_oauth2_application_success"))
	ctx.HTML(http.StatusOK, tplSettingsEditApplication)
}

// EditApplication response for editing oauth2 application
func EditApplication(ctx *context.Context) {
	app, err := auth.GetOAuth2ApplicationByID(ctx, ctx.ParamsInt64("id"))
	if err != nil {
		if auth.IsErrOAuthApplicationNotFound(err) {
			ctx.NotFound("Application not found", err)
			return
		}
		ctx.ServerError("GetOAuth2ApplicationByID", err)
		return
	}
	if app.UID != instanceOwnerUserID {
		ctx.NotFound("Application not found", nil)
		return
	}
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminApplications"] = true
	ctx.Data["App"] = app
	ctx.HTML(http.StatusOK, tplSettingsEditApplication)
}

// EditApplicationPost response for editing oauth2 application
func EditApplicationPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditOAuth2ApplicationForm)
	ctx.Data["Title"] = ctx.Tr("settings.applications")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminApplications"] = true

	if ctx.HasError() {
		apps, err := auth.GetOAuth2ApplicationsByUserID(ctx, instanceOwnerUserID)
		if err != nil {
			ctx.ServerError("GetOAuth2ApplicationsByUserID", err)
			return
		}
		ctx.Data["Applications"] = apps

		ctx.HTML(http.StatusOK, tplSettingsApplications)
		return
	}
	var err error
	if ctx.Data["App"], err = auth.UpdateOAuth2Application(auth.UpdateOAuth2ApplicationOptions{
		ID:           ctx.ParamsInt64("id"),
		Name:         form.Name,
		RedirectURIs: []string{form.RedirectURI},
		UserID:       instanceOwnerUserID,
	}); err != nil {
		ctx.ServerError("UpdateOAuth2Application", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.update_oauth2_application_success"))
	ctx.HTML(http.StatusOK, tplSettingsEditApplication)
}

// ApplicationsRegenerateSecret handles the post request for regenerating the secret
func ApplicationsRegenerateSecret(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsAdminApplications"] = true
	ctx.Data["PageIsAdmin"] = true

	app, err := auth.GetOAuth2ApplicationByID(ctx, ctx.ParamsInt64("id"))
	if err != nil {
		if auth.IsErrOAuthApplicationNotFound(err) {
			ctx.NotFound("Application not found", err)
			return
		}
		ctx.ServerError("GetOAuth2ApplicationByID", err)
		return
	}
	if app.UID != instanceOwnerUserID {
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
	ctx.HTML(http.StatusOK, tplSettingsEditApplication)
}

// DeleteApplication deletes the given oauth2 application
func DeleteApplication(ctx *context.Context) {
	if err := auth.DeleteOAuth2Application(ctx.FormInt64("id"), instanceOwnerUserID); err != nil {
		ctx.ServerError("DeleteOAuth2Application", err)
		return
	}
	log.Trace("OAuth2 Application deleted: %s", ctx.Doer.Name)

	ctx.Flash.Success(ctx.Tr("settings.remove_oauth2_application_success"))
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": fmt.Sprintf("%s/admin/applications", setting.AppSubURL),
	})
}
