// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

const (
	tplSettingsApplications templates.TplName = "user/settings/applications"
)

// Applications render manage access token page
func Applications(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.applications")
	ctx.Data["PageIsSettingsApplications"] = true

	loadApplicationsData(ctx)

	ctx.HTML(http.StatusOK, tplSettingsApplications)
}

// ApplicationsPost response for add user's access token
func ApplicationsPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewAccessTokenForm)
	ctx.Data["Title"] = ctx.Tr("settings_title")
	ctx.Data["PageIsSettingsApplications"] = true

	_ = ctx.Req.ParseForm()
	scope, err := auth_model.AccessTokenScopeFromForm(ctx.Req.Form).Normalize()
	if err != nil {
		ctx.ServerError("GetScope", err)
		return
	}
	if !scope.HasPermissionScope() {
		ctx.Flash.Error(ctx.Tr("settings.at_least_one_permission"), true)
	}

	if ctx.HasError() {
		loadApplicationsData(ctx)
		ctx.HTML(http.StatusOK, tplSettingsApplications)
		return
	}

	t := &auth_model.AccessToken{
		UID:   ctx.Doer.ID,
		Name:  form.Name,
		Scope: scope,
	}

	exist, err := auth_model.AccessTokenByNameExists(ctx, t)
	if err != nil {
		ctx.ServerError("AccessTokenByNameExists", err)
		return
	}
	if exist {
		ctx.Flash.Error(ctx.Tr("settings.generate_token_name_duplicate", t.Name))
		ctx.Redirect(setting.AppSubURL + "/user/settings/applications")
		return
	}

	// a token-authenticated request must not mint a token with a broader scope than its own, nor
	// drop the public-only restriction. Web routes accept basic-auth PATs/OAuth tokens too, so this
	// must mirror the REST API guard in routers/api/v1/user/app.go.
	if ctx.Data["IsApiToken"] == true {
		apiTokenScope, ok := ctx.Data["ApiTokenScope"].(auth_model.AccessTokenScope)
		if !ok {
			ctx.HTTPError(http.StatusForbidden, "the authenticating token has no scope")
			return
		}
		hasScope, err := apiTokenScope.CanCreateChildScope(t.Scope)
		if err != nil {
			ctx.ServerError("CanCreateChildScope", err)
			return
		}
		if !hasScope {
			ctx.HTTPError(http.StatusForbidden, "cannot create an access token with a broader scope than the authenticating token")
			return
		}
		if t.Scope, err = t.Scope.EnforcePublicOnlyFrom(apiTokenScope); err != nil {
			ctx.ServerError("EnforcePublicOnlyFrom", err)
			return
		}
	}

	if err := auth_model.NewAccessToken(ctx, t); err != nil {
		ctx.ServerError("NewAccessToken", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.generate_token_success"))
	ctx.Flash.Info(t.Token)

	ctx.Redirect(setting.AppSubURL + "/user/settings/applications")
}

// DeleteApplication response for delete user access token
func DeleteApplication(ctx *context.Context) {
	if err := auth_model.DeleteAccessTokenByID(ctx, ctx.FormInt64("id"), ctx.Doer.ID); err != nil {
		ctx.Flash.Error("DeleteAccessTokenByID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("settings.delete_token_success"))
	}

	ctx.JSONRedirect(setting.AppSubURL + "/user/settings/applications")
}

func loadApplicationsData(ctx *context.Context) {
	ctx.Data["AccessTokenScopePublicOnly"] = auth_model.AccessTokenScopePublicOnly
	tokens, err := db.Find[auth_model.AccessToken](ctx, auth_model.ListAccessTokensOptions{UserID: ctx.Doer.ID})
	if err != nil {
		ctx.ServerError("ListAccessTokens", err)
		return
	}
	ctx.Data["Tokens"] = tokens
	ctx.Data["EnableOAuth2"] = setting.OAuth2.Enabled

	// Handle specific ordered token categories for admin or non-admin users
	tokenCategoryNames := auth_model.GetAccessTokenCategories()
	if !ctx.Doer.IsAdmin {
		tokenCategoryNames = util.SliceRemoveAll(tokenCategoryNames, "admin")
	}
	ctx.Data["TokenCategories"] = tokenCategoryNames

	if setting.OAuth2.Enabled {
		ctx.Data["Applications"], err = db.Find[auth_model.OAuth2Application](ctx, auth_model.FindOAuth2ApplicationsOptions{
			OwnerID: ctx.Doer.ID,
		})
		if err != nil {
			ctx.ServerError("GetOAuth2ApplicationsByUserID", err)
			return
		}
		ctx.Data["Grants"], err = auth_model.GetOAuth2GrantsByUserID(ctx, ctx.Doer.ID)
		if err != nil {
			ctx.ServerError("GetOAuth2GrantsByUserID", err)
			return
		}
	}
}
