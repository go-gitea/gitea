// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"net/http"
	"strings"

	audit_model "gitea.dev/models/audit"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"

	"xorm.io/builder"
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
	var scopeNames []string
	const accessTokenScopePrefix = "scope-"
	for k, v := range ctx.Req.Form {
		if strings.HasPrefix(k, accessTokenScopePrefix) {
			scopeNames = append(scopeNames, v...)
		}
	}

	scope, err := auth_model.AccessTokenScope(strings.Join(scopeNames, ",")).Normalize()
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

	if err := auth_model.NewAccessToken(ctx, t); err != nil {
		ctx.ServerError("NewAccessToken", err)
		return
	}

	audit.Record(ctx, audit_model.UserAccessTokenAdd, ctx.Doer, ctx.Doer,
		fmt.Sprintf("Added access token %s for user %s with scope %s.", t.Name, ctx.Doer.Name, t.Scope), "token", t.Name, "scope", t.Scope)

	ctx.Flash.Success(ctx.Tr("settings.generate_token_success"))
	ctx.Flash.Info(t.Token)

	ctx.Redirect(setting.AppSubURL + "/user/settings/applications")
}

// DeleteApplication response for delete user access token
func DeleteApplication(ctx *context.Context) {
	t, exist, err := db.Get[auth_model.AccessToken](ctx, builder.Eq{"id": ctx.FormInt64("id"), "uid": ctx.Doer.ID})
	if err != nil {
		ctx.ServerError("GetAccessToken", err)
		return
	} else if !exist {
		ctx.Flash.Error("DeleteAccessTokenByID: not found")
		ctx.JSONRedirect(setting.AppSubURL + "/user/settings/applications")
		return
	}

	if err := auth_model.DeleteAccessTokenByID(ctx, t.ID, ctx.Doer.ID); err != nil {
		ctx.Flash.Error("DeleteAccessTokenByID: " + err.Error())
	} else {
		audit.Record(ctx, audit_model.UserAccessTokenRemove, ctx.Doer, ctx.Doer,
			fmt.Sprintf("Removed access token %s from user %s.", t.Name, ctx.Doer.Name), "token", t.Name)

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
