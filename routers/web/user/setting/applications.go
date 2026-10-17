// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"strings"

	audit_model "gitea.dev/models/audit"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

const (
	tplSettingsApplications templates.TplName = "user/settings/applications"
	tplAccessTokens         templates.TplName = "shared/user/access_tokens"
)

// Applications render manage access token page
func Applications(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.applications")
	ctx.Data["PageIsSettingsApplications"] = true

	loadApplicationsData(ctx)

	ctx.HTML(http.StatusOK, tplSettingsApplications)
}

type AccessTokensPanel struct {
	Tokens          []*auth_model.AccessToken
	ScopeCategories []string
	ScopePublicOnly auth_model.AccessTokenScope
	Link            string
	IsBot           bool
	NewTokenValue   string
}

func NewAccessTokensPanel(ctx *context.Context, owner *user_model.User, link string) *AccessTokensPanel {
	tokens, err := db.Find[auth_model.AccessToken](ctx, auth_model.ListAccessTokensOptions{UserID: owner.ID})
	if err != nil {
		ctx.ServerError("ListAccessTokens", err)
		return nil
	}
	panel := &AccessTokensPanel{
		Tokens:          tokens,
		ScopeCategories: auth_model.GetAccessTokenCategories(),
		ScopePublicOnly: auth_model.AccessTokenScopePublicOnly,
		Link:            link,
		IsBot:           owner.IsTypeBot(),
	}
	if !owner.IsAdmin {
		panel.ScopeCategories = util.SliceRemoveAll(panel.ScopeCategories, "admin")
	}
	return panel
}

// CreateAccessToken handles the panel's create form, which posts to the panel link
func CreateAccessToken(ctx *context.Context, owner *user_model.User) {
	form := context.GetFetchActionForm[*forms.NewAccessTokenForm](ctx)
	if form == nil {
		return
	}

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
		ctx.JSONError(ctx.Tr("settings.at_least_one_permission"))
		return
	}

	t := &auth_model.AccessToken{
		UID:   owner.ID,
		Name:  form.Name,
		Scope: scope,
	}

	exist, err := auth_model.AccessTokenByNameExists(ctx, t)
	if err != nil {
		ctx.ServerError("AccessTokenByNameExists", err)
		return
	}
	if exist {
		ctx.JSONErrorWithField(ctx.Tr("settings.generate_token_name_duplicate", t.Name), "name")
		return
	}

	// a token-authenticated request must not mint a token with a broader scope than its own, nor
	// drop the public-only restriction. Web routes accept basic-auth PATs/OAuth tokens too, so this
	// must mirror the REST API guard in routers/api/v1/user/app.go.
	apiTokenScope, hasApiTokenScope := ctx.Data["ApiTokenScope"].(auth_model.AccessTokenScope)
	if hasApiTokenScope {
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

	audit.Record(ctx, audit_model.UserAccessTokenAdd, owner, "token", t.Name, "token_scope", t.Scope)

	panel := NewAccessTokensPanel(ctx, owner, ctx.Link)
	if ctx.Written() {
		return
	}
	panel.NewTokenValue = t.Token
	if err := ctx.Render.HTML(ctx.Resp, http.StatusOK, tplAccessTokens, panel, ctx.TemplateContext); err != nil {
		ctx.ServerError("Render", err)
	}
}

// ApplicationsPost response for add user's access token
func ApplicationsPost(ctx *context.Context) {
	CreateAccessToken(ctx, ctx.Doer)
}

// DeleteApplication response for delete user access token
func DeleteApplication(ctx *context.Context) {
	DeleteAccessToken(ctx, ctx.Doer)
}

func DeleteAccessToken(ctx *context.Context, owner *user_model.User) {
	t, err := auth_model.GetAccessTokenByID(ctx, ctx.FormInt64("id"), owner.ID)
	if err != nil {
		ctx.Flash.Error("GetAccessTokenByID: " + err.Error())
	} else if err := auth_model.DeleteAccessTokenByID(ctx, t.ID, owner.ID); err != nil {
		ctx.Flash.Error("DeleteAccessTokenByID: " + err.Error())
	} else {
		audit.Record(ctx, audit_model.UserAccessTokenRemove, owner, "token", t.Name)

		ctx.Flash.Success(ctx.Tr("settings.delete_token_success"))
	}

	ctx.JSONRedirect("")
}

// RegenerateAccessToken response for regenerating a user's access token
func RegenerateAccessToken(ctx *context.Context) {
	t, err := auth_model.RegenerateAccessToken(ctx, ctx.FormInt64("id"), ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("RegenerateAccessToken", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.generate_token_success"))
	ctx.Flash.Info(t.Token)
	ctx.JSONRedirect(setting.AppSubURL + "/user/settings/applications")
}

func loadApplicationsData(ctx *context.Context) {
	ctx.Data["AccessTokens"] = NewAccessTokensPanel(ctx, ctx.Doer, ctx.Link)
	if ctx.Written() {
		return
	}
	ctx.Data["EnableOAuth2"] = setting.OAuth2.Enabled

	if setting.OAuth2.Enabled {
		var err error
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
