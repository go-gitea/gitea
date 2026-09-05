// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
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

var (
	// ErrAccessTokenNoPermission is returned when the submitted scope grants no permission at all
	ErrAccessTokenNoPermission = errors.New("access token has no permission scope")
	// ErrAccessTokenNameDuplicate is returned when the owner already has a token of that name
	ErrAccessTokenNameDuplicate = errors.New("access token name already exists")
	// ErrAccessTokenAdminScope is returned when an admin scope is requested for an owner that can never be a site administrator
	ErrAccessTokenAdminScope = errors.New("access token cannot carry an admin scope")
	// ErrAccessTokenScopeEscalation is returned when the authenticating token is narrower than the token it asks for
	ErrAccessTokenScopeEscalation = errors.New("cannot create an access token with a broader scope than the authenticating token")
)

// NewAccessTokenFromForm creates an access token for owner from the submitted scope form.
// Pass allowAdminScope=false for owners that can never be a site administrator.
func NewAccessTokenFromForm(ctx *context.Context, owner *user_model.User, name string, allowAdminScope bool) (*auth_model.AccessToken, error) {
	_ = ctx.Req.ParseForm()
	scope, err := forms.AccessTokenScopeFromForm(ctx.Req.Form).Normalize()
	if err != nil {
		return nil, err
	}
	if !scope.HasPermissionScope() {
		return nil, ErrAccessTokenNoPermission
	}
	if !allowAdminScope {
		hasAdminScope, err := scope.HasAnyScope(auth_model.AccessTokenScopeReadAdmin, auth_model.AccessTokenScopeWriteAdmin)
		if err != nil {
			return nil, err
		}
		if hasAdminScope {
			return nil, ErrAccessTokenAdminScope
		}
	}

	t := &auth_model.AccessToken{
		UID:   owner.ID,
		Name:  name,
		Scope: scope,
	}

	exist, err := auth_model.AccessTokenByNameExists(ctx, t)
	if err != nil {
		return nil, err
	}
	if exist {
		return nil, ErrAccessTokenNameDuplicate
	}

	// a token-authenticated request must not mint a token with a broader scope than its own, nor
	// drop the public-only restriction; mirrors the REST API guard in routers/api/v1/user/app.go
	// for the day a token-auth path reaches here
	if apiTokenScope, ok := ctx.Data["ApiTokenScope"].(auth_model.AccessTokenScope); ok {
		hasScope, err := apiTokenScope.CanCreateChildScope(t.Scope)
		if err != nil {
			return nil, err
		}
		if !hasScope {
			return nil, ErrAccessTokenScopeEscalation
		}
		if t.Scope, err = t.Scope.EnforcePublicOnlyFrom(apiTokenScope); err != nil {
			return nil, err
		}
	}

	if err := auth_model.NewAccessToken(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// ApplicationsPost response for add user's access token
//
// The create form is a "form-fetch-action" form, so validation problems are answered
// with ctx.JSONError (the client shows a toast and keeps the submitted form state);
// success still redirects so the fresh one-time token renders in the flash message.
func ApplicationsPost(ctx *context.Context) {
	form := context.GetFetchActionForm[*forms.NewAccessTokenForm](ctx)
	if form == nil {
		return
	}

	// a non-admin may still hold an admin-scoped token: it stays inert until they become one
	t, err := NewAccessTokenFromForm(ctx, ctx.Doer, form.Name, true)
	switch {
	case errors.Is(err, ErrAccessTokenNoPermission):
		ctx.JSONError(ctx.Tr("settings.at_least_one_permission"))
	case errors.Is(err, ErrAccessTokenNameDuplicate):
		ctx.JSONErrorWithField(ctx.Tr("settings.generate_token_name_duplicate", form.Name), "name")
	case errors.Is(err, ErrAccessTokenScopeEscalation):
		ctx.HTTPError(http.StatusForbidden, err.Error())
	case err != nil:
		ctx.ServerError("NewAccessTokenFromForm", err)
	default:
		ctx.Flash.Success(ctx.Tr("settings.generate_token_success"))
		ctx.Flash.Info(t.Token)
		ctx.Redirect(setting.AppSubURL + "/user/settings/applications")
	}
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
