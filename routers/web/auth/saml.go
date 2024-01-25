// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/auth/source/saml"
	"code.gitea.io/gitea/services/externalaccount"

	"github.com/markbates/goth"
)

func SignInSAML(ctx *context.Context) {
	provider := ctx.Params(":provider")

	loginSource, err := auth.GetActiveAuthSourceByName(ctx, provider, auth.SAML)
	if err != nil || loginSource == nil {
		ctx.NotFound("SAMLMetadata", err)
		return
	}

	if err = loginSource.Cfg.(*saml.Source).Callout(ctx.Req, ctx.Resp); err != nil {
		if strings.Contains(err.Error(), "no provider for ") {
			ctx.Error(http.StatusNotFound)
			return
		}
		ctx.ServerError("SignIn", err)
	}
}

func SignInSAMLCallback(ctx *context.Context) {
	provider := ctx.Params(":provider")
	loginSource, err := auth.GetActiveAuthSourceByName(ctx, provider, auth.SAML)
	if err != nil || loginSource == nil {
		ctx.NotFound("SignInSAMLCallback", err)
		return
	}

	if loginSource == nil {
		ctx.ServerError("SignIn", fmt.Errorf("no valid provider found, check configured callback url in provider"))
		return
	}

	u, gothUser, err := samlUserLoginCallback(*ctx, loginSource, ctx.Req, ctx.Resp)
	if err != nil {
		ctx.ServerError("SignInSAMLCallback", err)
		return
	}

	if u == nil {
		if ctx.Doer != nil {
			// attach user to already logged in user
			err = externalaccount.LinkAccountToUser(ctx, ctx.Doer, gothUser, auth.SAML)
			if err != nil {
				ctx.ServerError("LinkAccountToUser", err)
				return
			}

			ctx.Redirect(setting.AppSubURL + "/user/settings/security")
			return
		} else if !setting.Service.AllowOnlyInternalRegistration && false {
			// TODO: allow auto registration from saml users (OAuth2 uses the following setting.OAuth2Client.EnableAutoRegistration)
		} else {
			// no existing user is found, request attach or new account
			showLinkingLogin(ctx, gothUser, auth.SAML)
			return
		}
	}

	handleSamlSignIn(ctx, loginSource, u, gothUser)
}

func handleSamlSignIn(ctx *context.Context, source *auth.Source, u *user_model.User, gothUser goth.User) {
	if err := updateSession(ctx, nil, map[string]any{
		"uid":   u.ID,
		"uname": u.Name,
	}); err != nil {
		ctx.ServerError("updateSession", err)
		return
	}

	// Clear whatever CSRF cookie has right now, force to generate a new one
	ctx.Csrf.DeleteCookie(ctx)

	// Register last login
	u.SetLastLogin()

	// update external user information
	if err := externalaccount.UpdateExternalUser(ctx, u, gothUser, auth.SAML); err != nil {
		if !errors.Is(err, util.ErrNotExist) {
			log.Error("UpdateExternalUser failed: %v", err)
		}
	}

	if err := resetLocale(ctx, u); err != nil {
		ctx.ServerError("resetLocale", err)
		return
	}

	if redirectTo := ctx.GetSiteCookie("redirect_to"); len(redirectTo) > 0 {
		middleware.DeleteRedirectToCookie(ctx.Resp)
		ctx.RedirectToFirst(redirectTo)
		return
	}

	ctx.Redirect(setting.AppSubURL + "/")
}

func samlUserLoginCallback(ctx context.Context, authSource *auth.Source, request *http.Request, response http.ResponseWriter) (*user_model.User, goth.User, error) {
	samlSource := authSource.Cfg.(*saml.Source)

	gothUser, err := samlSource.Callback(request, response)
	if err != nil {
		return nil, gothUser, err
	}

	user := &user_model.User{
		LoginName:   gothUser.UserID,
		LoginType:   auth.SAML,
		LoginSource: authSource.ID,
	}

	hasUser, err := user_model.GetUser(ctx, user)
	if err != nil {
		return nil, goth.User{}, err
	}

	if hasUser {
		return user, gothUser, nil
	}

	// search in external linked users
	externalLoginUser := &user_model.ExternalLoginUser{
		ExternalID:    gothUser.UserID,
		LoginSourceID: authSource.ID,
	}
	hasUser, err = user_model.GetExternalLogin(ctx, externalLoginUser)
	if err != nil {
		return nil, goth.User{}, err
	}
	if hasUser {
		user, err = user_model.GetUserByID(request.Context(), externalLoginUser.UserID)
		return user, gothUser, err
	}

	// no user found to login
	return nil, gothUser, nil
}

func SAMLMetadata(ctx *context.Context) {
	provider := ctx.Params(":provider")
	loginSource, err := auth.GetActiveAuthSourceByName(ctx, provider, auth.SAML)
	if err != nil || loginSource == nil {
		ctx.NotFound("SAMLMetadata", err)
		return
	}
	if err = loginSource.Cfg.(*saml.Source).Metadata(ctx.Req, ctx.Resp); err != nil {
		ctx.ServerError("SAMLMetadata", err)
	}
}
