// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/session"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/stretchr/testify/assert"
)

func addOAuth2Source(t *testing.T, authName string, cfg oauth2.Source) {
	cfg.Provider = util.IfZero(cfg.Provider, "gitea")
	err := auth_model.CreateSource(db.DefaultContext, &auth_model.Source{
		Type:     auth_model.OAuth2,
		Name:     authName,
		IsActive: true,
		Cfg:      &cfg,
	})
	assert.NoError(t, err)
}

func TestUserLogin(t *testing.T) {
	ctx, resp := contexttest.MockContext(t, "/user/login")
	SignIn(ctx)
	assert.Equal(t, http.StatusOK, resp.Code)

	ctx, resp = contexttest.MockContext(t, "/user/login")
	ctx.IsSigned = true
	SignIn(ctx)
	assert.Equal(t, http.StatusSeeOther, resp.Code)
	assert.Equal(t, "/", test.RedirectURL(resp))

	ctx, resp = contexttest.MockContext(t, "/user/login?redirect_to=/other")
	ctx.IsSigned = true
	SignIn(ctx)
	assert.Equal(t, "/other", test.RedirectURL(resp))

	ctx, resp = contexttest.MockContext(t, "/user/login")
	ctx.Req.AddCookie(&http.Cookie{Name: "redirect_to", Value: "/other-cookie"})
	ctx.IsSigned = true
	SignIn(ctx)
	assert.Equal(t, "/other-cookie", test.RedirectURL(resp))

	ctx, resp = contexttest.MockContext(t, "/user/login?redirect_to="+url.QueryEscape("https://example.com"))
	ctx.IsSigned = true
	SignIn(ctx)
	assert.Equal(t, "/", test.RedirectURL(resp))
}

func TestSignUpOAuth2ButMissingFields(t *testing.T) {
	defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
	defer test.MockVariableValue(&gothic.CompleteUserAuth, func(res http.ResponseWriter, req *http.Request) (goth.User, error) {
		return goth.User{Provider: "dummy-auth-source", UserID: "dummy-user"}, nil
	})()

	addOAuth2Source(t, "dummy-auth-source", oauth2.Source{})

	mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockStore("dummy-sid")}
	ctx, resp := contexttest.MockContext(t, "/user/oauth2/dummy-auth-source/callback?code=dummy-code", mockOpt)
	ctx.SetPathParam("provider", "dummy-auth-source")
	SignInOAuthCallback(ctx)
	assert.Equal(t, http.StatusSeeOther, resp.Code)
	assert.Equal(t, "/user/link_account", test.RedirectURL(resp))

	// then the user will be redirected to the link account page, and see a message about the missing fields
	ctx, _ = contexttest.MockContext(t, "/user/link_account", mockOpt)
	LinkAccount(ctx)
	assert.EqualValues(t, "auth.oauth_callback_unable_auto_reg:dummy-auth-source,email", ctx.Data["AutoRegistrationFailedPrompt"])
}
