// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

type OAuth2CommonHandlers struct {
	OwnerID            int64             // 0 for instance-wide, otherwise OrgID or UserID
	BasePathList       string            // the base URL for the application list page, eg: "/user/setting/applications"
	BasePathEditPrefix string            // the base URL for the application edit page, will be appended with app id, eg: "/user/setting/applications/oauth2"
	TplAppEdit         templates.TplName // the template for the application edit page
}

func (oa *OAuth2CommonHandlers) renderEditPage(ctx *context.Context) {
	app := ctx.Data["App"].(*auth.OAuth2Application)
	ctx.Data["FormActionPath"] = fmt.Sprintf("%s/%d", oa.BasePathEditPrefix, app.ID)

	if ctx.ContextUser != nil && ctx.ContextUser.IsOrganization() {
		if err := shared_user.LoadHeaderCount(ctx); err != nil {
			ctx.ServerError("LoadHeaderCount", err)
			return
		}
	}

	ctx.HTML(http.StatusOK, oa.TplAppEdit)
}

// AddApp adds an oauth2 application
func (oa *OAuth2CommonHandlers) AddApp(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditOAuth2ApplicationForm)
	if ctx.HasError() {
		ctx.Flash.Error(ctx.GetErrMsg())
		// go to the application list page
		ctx.Redirect(oa.BasePathList)
		return
	}

	app, err := auth.CreateOAuth2Application(ctx, auth.CreateOAuth2ApplicationOptions{
		Name:                       form.Name,
		RedirectURIs:               util.SplitTrimSpace(form.RedirectURIs, "\n"),
		UserID:                     oa.OwnerID,
		ConfidentialClient:         form.ConfidentialClient,
		SkipSecondaryAuthorization: form.SkipSecondaryAuthorization,
	})
	if err != nil {
		ctx.ServerError("CreateOAuth2Application", err)
		return
	}

	// render the edit page with secret
	ctx.Flash.Success(ctx.Tr("settings.create_oauth2_application_success"), true)
	ctx.Data["App"] = app
	ctx.Data["ClientSecret"], err = app.GenerateClientSecret(ctx)
	if err != nil {
		ctx.ServerError("GenerateClientSecret", err)
		return
	}

	oa.renderEditPage(ctx)
}

// EditShow displays the given application
func (oa *OAuth2CommonHandlers) EditShow(ctx *context.Context) {
	app, err := auth.GetOAuth2ApplicationByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if auth.IsErrOAuthApplicationNotFound(err) {
			ctx.NotFound(err)
			return
		}
		ctx.ServerError("GetOAuth2ApplicationByID", err)
		return
	}
	if app.UID != oa.OwnerID {
		ctx.NotFound(nil)
		return
	}
	ctx.Data["App"] = app
	oa.renderEditPage(ctx)
}

// EditSave saves the oauth2 application
func (oa *OAuth2CommonHandlers) EditSave(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditOAuth2ApplicationForm)

	if ctx.HasError() {
		app, err := auth.GetOAuth2ApplicationByID(ctx, ctx.PathParamInt64("id"))
		if err != nil {
			if auth.IsErrOAuthApplicationNotFound(err) {
				ctx.NotFound(err)
				return
			}
			ctx.ServerError("GetOAuth2ApplicationByID", err)
			return
		}
		if app.UID != oa.OwnerID {
			ctx.NotFound(nil)
			return
		}
		ctx.Data["App"] = app

		oa.renderEditPage(ctx)
		return
	}

	var err error
	if ctx.Data["App"], err = auth.UpdateOAuth2Application(ctx, auth.UpdateOAuth2ApplicationOptions{
		ID:                         ctx.PathParamInt64("id"),
		Name:                       form.Name,
		RedirectURIs:               util.SplitTrimSpace(form.RedirectURIs, "\n"),
		UserID:                     oa.OwnerID,
		ConfidentialClient:         form.ConfidentialClient,
		SkipSecondaryAuthorization: form.SkipSecondaryAuthorization,
	}); err != nil {
		ctx.ServerError("UpdateOAuth2Application", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.update_oauth2_application_success"))
	ctx.Redirect(oa.BasePathList)
}

// RegenerateSecret regenerates the secret
func (oa *OAuth2CommonHandlers) RegenerateSecret(ctx *context.Context) {
	app, err := auth.GetOAuth2ApplicationByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if auth.IsErrOAuthApplicationNotFound(err) {
			ctx.NotFound(err)
			return
		}
		ctx.ServerError("GetOAuth2ApplicationByID", err)
		return
	}
	if app.UID != oa.OwnerID {
		ctx.NotFound(nil)
		return
	}
	ctx.Data["App"] = app
	ctx.Data["ClientSecret"], err = app.GenerateClientSecret(ctx)
	if err != nil {
		ctx.ServerError("GenerateClientSecret", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.update_oauth2_application_success"), true)
	oa.renderEditPage(ctx)
}

// DeleteApp deletes the given oauth2 application
func (oa *OAuth2CommonHandlers) DeleteApp(ctx *context.Context) {
	if err := auth.DeleteOAuth2Application(ctx, ctx.PathParamInt64("id"), oa.OwnerID); err != nil {
		ctx.ServerError("DeleteOAuth2Application", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.remove_oauth2_application_success"))
	ctx.JSONRedirect(oa.BasePathList)
}

// RevokeGrant revokes the grant
func (oa *OAuth2CommonHandlers) RevokeGrant(ctx *context.Context) {
	if err := auth.RevokeOAuth2Grant(ctx, ctx.PathParamInt64("grantId"), oa.OwnerID); err != nil {
		ctx.ServerError("RevokeOAuth2Grant", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.revoke_oauth2_grant_success"))
	ctx.JSONRedirect(oa.BasePathList)
}
