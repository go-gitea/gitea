// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/session"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func addOAuth2Source(t *testing.T, authName string, cfg oauth2.Source) {
	cfg.Provider = util.IfZero(cfg.Provider, "gitea")
	err := auth_model.CreateSource(t.Context(), &auth_model.Source{
		Type:     auth_model.OAuth2,
		Name:     authName,
		IsActive: true,
		Cfg:      &cfg,
	})
	require.NoError(t, err)
}

func TestWebAuthUserLogin(t *testing.T) {
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

func TestWebAuthOAuth2(t *testing.T) {
	defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()

	_ = oauth2.Init(t.Context())
	addOAuth2Source(t, "dummy+auth's source", oauth2.Source{})

	t.Run("OAuth2MissingField", func(t *testing.T) {
		defer test.MockVariableValue(&gothic.CompleteUserAuth, func(res http.ResponseWriter, req *http.Request) (goth.User, error) {
			return goth.User{Provider: "dummy+auth's source", UserID: "dummy-user"}, nil
		})()
		mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockMemStore("dummy-sid")}
		ctx, resp := contexttest.MockContext(t, "/user/oauth2/..../callback?code=dummy-code", mockOpt)
		ctx.SetPathParamRaw("provider", "dummy+auth%27s%20source")
		SignInOAuthCallback(ctx)
		assert.Equal(t, http.StatusSeeOther, resp.Code)
		assert.Equal(t, "/user/link_account", test.RedirectURL(resp))

		// then the user will be redirected to the link account page, and see a message about the missing fields
		ctx, _ = contexttest.MockContext(t, "/user/link_account", mockOpt)
		LinkAccount(ctx)
		assert.Equal(t, template.HTML("auth.oauth_callback_unable_auto_reg:dummy+auth&#39;s source,email"), ctx.Data["AutoRegistrationFailedPrompt"])
	})

	t.Run("OAuth2CallbackError", func(t *testing.T) {
		mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockMemStore("dummy-sid")}
		ctx, resp := contexttest.MockContext(t, "/user/oauth2/...../callback", mockOpt)
		ctx.SetPathParamRaw("provider", "dummy+auth%27s%20source")
		SignInOAuthCallback(ctx)
		assert.Equal(t, http.StatusSeeOther, resp.Code)
		assert.Equal(t, "/user/login", test.RedirectURL(resp))
		assert.Contains(t, ctx.Flash.ErrorMsg, "auth.oauth.signin.error.general")
	})

	t.Run("RedirectSingleProvider", func(t *testing.T) {
		enablePassword := &setting.Service.EnablePasswordSignInForm
		enableOpenID := &setting.Service.EnableOpenIDSignIn
		enablePasskey := &setting.Service.EnablePasskeyAuth
		defer test.MockVariableValue(enablePassword, false)()
		defer test.MockVariableValue(enableOpenID, false)()
		defer test.MockVariableValue(enablePasskey, false)()

		testSignIn := func(t *testing.T, link string, expectedCode int, expectedRedirect string) {
			ctx, resp := contexttest.MockContext(t, link)
			SignIn(ctx)
			assert.Equal(t, expectedCode, resp.Code)
			if expectedCode == http.StatusSeeOther {
				assert.Equal(t, expectedRedirect, test.RedirectURL(resp))
			}
		}
		testSignIn(t, "/user/login", http.StatusSeeOther, "/user/oauth2/dummy+auth%27s%20source")
		testSignIn(t, "/user/login?redirect_to=/", http.StatusSeeOther, "/user/oauth2/dummy+auth%27s%20source?redirect_to=%2F")

		*enablePassword, *enableOpenID, *enablePasskey = true, false, false
		testSignIn(t, "/user/login", http.StatusOK, "")
		*enablePassword, *enableOpenID, *enablePasskey = false, true, false
		testSignIn(t, "/user/login", http.StatusOK, "")
		*enablePassword, *enableOpenID, *enablePasskey = false, false, true
		testSignIn(t, "/user/login", http.StatusOK, "")

		*enablePassword, *enableOpenID, *enablePasskey = false, false, false
		addOAuth2Source(t, "dummy-auth-source-2", oauth2.Source{})
		testSignIn(t, "/user/login", http.StatusOK, "")
	})

	t.Run("OIDCLogout", func(t *testing.T) {
		var mockServer *httptest.Server
		mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/.well-known/openid-configuration":
				_, _ = w.Write([]byte(`{
				"issuer": "` + mockServer.URL + `",
				"authorization_endpoint": "` + mockServer.URL + `/authorize",
				"token_endpoint": "` + mockServer.URL + `/token",
				"userinfo_endpoint": "` + mockServer.URL + `/userinfo",
				"end_session_endpoint": "https://example.com/oidc-logout?oidc-key=oidc-val"
			}`))
			default:
				http.NotFound(w, r)
			}
		}))
		defer mockServer.Close()

		addOAuth2Source(t, "oidc-auth-source", oauth2.Source{
			Provider:                      "openidConnect",
			ClientID:                      "mock-client-id",
			OpenIDConnectAutoDiscoveryURL: mockServer.URL + "/.well-known/openid-configuration",
		})
		authSource, err := auth_model.GetActiveOAuth2SourceByAuthName(t.Context(), "oidc-auth-source")
		require.NoError(t, err)

		mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockMemStore("dummy-sid")}
		ctx, resp := contexttest.MockContext(t, "/user/logout", mockOpt)
		ctx.Doer = &user_model.User{ID: 1, LoginType: auth_model.OAuth2, LoginSource: authSource.ID}
		SignOut(ctx)
		assert.Equal(t, http.StatusSeeOther, resp.Code)
		u, err := url.Parse(test.RedirectURL(resp))
		require.NoError(t, err)
		expectedValues := url.Values{"oidc-key": []string{"oidc-val"}, "post_logout_redirect_uri": []string{setting.AppURL}, "client_id": []string{"mock-client-id"}}
		assert.Equal(t, expectedValues, u.Query())
		u.RawQuery = ""
		assert.Equal(t, "https://example.com/oidc-logout", u.String())
	})
}
