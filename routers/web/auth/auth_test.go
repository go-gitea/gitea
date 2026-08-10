// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/session"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"
	auth_service "gitea.dev/services/auth"
	"gitea.dev/services/auth/source/oauth2"
	"gitea.dev/services/contexttest"

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

		oauthUser := &user_model.User{ID: 1, LoginType: auth_model.OAuth2, LoginSource: authSource.ID}

		t.Run("OAuth2SignInRedirectsToOIDC", func(t *testing.T) {
			mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockMemStore("dummy-sid-oauth")}
			ctx, resp := contexttest.MockContext(t, "/user/logout", mockOpt)
			ctx.Doer = oauthUser
			require.NoError(t, ctx.Session.Set(session.KeySignInMethod, session.SignInMethodOAuth2))
			SignOut(ctx)
			assert.Equal(t, http.StatusSeeOther, resp.Code)
			u, err := url.Parse(test.RedirectURL(resp))
			require.NoError(t, err)
			expectedValues := url.Values{"oidc-key": []string{"oidc-val"}, "post_logout_redirect_uri": []string{setting.AppURL}, "client_id": []string{"mock-client-id"}}
			assert.Equal(t, expectedValues, u.Query())
			u.RawQuery = ""
			assert.Equal(t, "https://example.com/oidc-logout", u.String())
		})

		t.Run("OAuth2SignInIncludesIDTokenHint", func(t *testing.T) {
			mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockMemStore("dummy-sid-oauth-idtoken")}
			ctx, resp := contexttest.MockContext(t, "/user/logout", mockOpt)
			ctx.Doer = oauthUser
			require.NoError(t, ctx.Session.Set(session.KeySignInMethod, session.SignInMethodOAuth2))
			require.NoError(t, ctx.Session.Set(session.KeyOIDCIDToken, "mock-id-token"))
			SignOut(ctx)
			assert.Equal(t, http.StatusSeeOther, resp.Code)
			u, err := url.Parse(test.RedirectURL(resp))
			require.NoError(t, err)
			expectedValues := url.Values{"oidc-key": []string{"oidc-val"}, "post_logout_redirect_uri": []string{setting.AppURL}, "client_id": []string{"mock-client-id"}, "id_token_hint": []string{"mock-id-token"}}
			assert.Equal(t, expectedValues, u.Query())
		})

		t.Run("OAuth2CallbackStoresIDTokenForLogout", func(t *testing.T) {
			defer test.MockVariableValue(&gothic.CompleteUserAuth, func(res http.ResponseWriter, req *http.Request) (goth.User, error) {
				return goth.User{Provider: "oidc-auth-source", UserID: "oidc-new-user", Email: "oidc-new-user@example.com", IDToken: "real-flow-id-token"}, nil
			})()

			mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockMemStore("dummy-sid-oidc-callback")}
			ctx, resp := contexttest.MockContext(t, "/user/oauth2/..../callback?code=dummy-code", mockOpt)
			ctx.SetPathParamRaw("provider", "oidc-auth-source")
			SignInOAuthCallback(ctx)
			require.Equal(t, http.StatusSeeOther, resp.Code)
			assert.Equal(t, "real-flow-id-token", ctx.Session.Get(session.KeyOIDCIDToken))

			// signing out the same session must carry the id_token through as id_token_hint
			ctx, resp = contexttest.MockContext(t, "/user/logout", mockOpt)
			ctx.Doer = oauthUser
			SignOut(ctx)
			assert.Equal(t, http.StatusSeeOther, resp.Code)
			u, err := url.Parse(test.RedirectURL(resp))
			require.NoError(t, err)
			assert.Equal(t, "real-flow-id-token", u.Query().Get("id_token_hint"))
		})

		t.Run("OAuth2CallbackWithoutIDTokenOmitsHint", func(t *testing.T) {
			defer test.MockVariableValue(&gothic.CompleteUserAuth, func(res http.ResponseWriter, req *http.Request) (goth.User, error) {
				return goth.User{Provider: "oidc-auth-source", UserID: "oidc-new-user-2", Email: "oidc-new-user-2@example.com"}, nil
			})()

			mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockMemStore("dummy-sid-oidc-callback-no-token")}
			ctx, resp := contexttest.MockContext(t, "/user/oauth2/..../callback?code=dummy-code", mockOpt)
			ctx.SetPathParamRaw("provider", "oidc-auth-source")
			SignInOAuthCallback(ctx)
			require.Equal(t, http.StatusSeeOther, resp.Code)
			assert.Empty(t, ctx.Session.Get(session.KeyOIDCIDToken))

			ctx, resp = contexttest.MockContext(t, "/user/logout", mockOpt)
			ctx.Doer = oauthUser
			SignOut(ctx)
			assert.Equal(t, http.StatusSeeOther, resp.Code)
			u, err := url.Parse(test.RedirectURL(resp))
			require.NoError(t, err)
			assert.Empty(t, u.Query().Get("id_token_hint"))
		})

		t.Run("OAuth2CallbackClearsStaleIDTokenOnRegen", func(t *testing.T) {
			// simulates a user who signed in before with an id_token (stale value already
			// in the session), then signs in again via a callback that yields no id_token
			mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockMemStore("dummy-sid-oidc-stale-token")}
			ctx, resp := contexttest.MockContext(t, "/user/oauth2/..../callback?code=dummy-code", mockOpt)
			require.NoError(t, ctx.Session.Set(session.KeyOIDCIDToken, "stale-id-token-from-prior-login"))
			ctx.Doer = oauthUser
			handleOAuth2SignIn(ctx, authSource, oauthUser, goth.User{Provider: "oidc-auth-source", UserID: "oauth-user"})
			require.Equal(t, http.StatusSeeOther, resp.Code)
			assert.Empty(t, ctx.Session.Get(session.KeyOIDCIDToken), "stale id_token from a prior session must not survive regenerateSession")

			ctx, resp = contexttest.MockContext(t, "/user/logout", mockOpt)
			ctx.Doer = oauthUser
			SignOut(ctx)
			assert.Equal(t, http.StatusSeeOther, resp.Code)
			u, err := url.Parse(test.RedirectURL(resp))
			require.NoError(t, err)
			assert.Empty(t, u.Query().Get("id_token_hint"), "logout must not reuse a stale id_token as id_token_hint")
		})

		t.Run("OAuth2CallbackClearsStaleIDTokenOnRegenWith2FA", func(t *testing.T) {
			// same as OAuth2CallbackClearsStaleIDTokenOnRegen, but through the 2FA-required
			// branch in handleOAuth2SignIn, a separate call site from the !needs2FA one
			tfa := &auth_model.TwoFactor{UID: oauthUser.ID}
			require.NoError(t, auth_model.NewTwoFactor(t.Context(), tfa))
			t.Cleanup(func() { _ = auth_model.DeleteTwoFactorByID(t.Context(), tfa.ID, oauthUser.ID) })

			mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockMemStore("dummy-sid-oidc-stale-token-2fa")}
			ctx, resp := contexttest.MockContext(t, "/user/oauth2/..../callback?code=dummy-code", mockOpt)
			require.NoError(t, ctx.Session.Set(session.KeyOIDCIDToken, "stale-id-token-from-prior-login"))
			ctx.Doer = oauthUser
			handleOAuth2SignIn(ctx, authSource, oauthUser, goth.User{Provider: "oidc-auth-source", UserID: "oauth-user"})
			require.Equal(t, http.StatusSeeOther, resp.Code)
			assert.Equal(t, "/user/two_factor", test.RedirectURL(resp))
			assert.Empty(t, ctx.Session.Get(session.KeyOIDCIDToken), "stale id_token must not survive the 2FA-required regeneration branch")
		})

		t.Run("PasswordSignInAfterOIDCClearsStaleIDTokenAndMethod", func(t *testing.T) {
			// a session that previously authenticated via OIDC (KeySignInMethod + KeyOIDCIDToken
			// set) must not keep redirecting to end_session_endpoint with a stale hint once the
			// same browser session re-authenticates via password: handleSignInFull's regenerateSession
			// call doesn't pass either key, so ClearSessionKeysForSignIn must be the one clearing them
			mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockMemStore("dummy-sid-password-after-oidc")}
			ctx, _ := contexttest.MockContext(t, "/user/login", mockOpt)
			require.NoError(t, ctx.Session.Set(session.KeySignInMethod, session.SignInMethodOAuth2))
			require.NoError(t, ctx.Session.Set(session.KeyOIDCIDToken, "stale-id-token-from-prior-oidc-login"))

			handleSignInFull(ctx, oauthUser, false)
			assert.NotEqual(t, session.SignInMethodOAuth2, ctx.Session.Get(session.KeySignInMethod), "password sign-in must not leave a stale OAuth2 sign-in method behind")
			assert.Empty(t, ctx.Session.Get(session.KeyOIDCIDToken), "password sign-in must not leave a stale id_token behind")

			ctx, resp := contexttest.MockContext(t, "/user/logout", mockOpt)
			ctx.Doer = oauthUser
			SignOut(ctx)
			assert.Equal(t, http.StatusSeeOther, resp.Code)
			assert.Equal(t, "/", test.RedirectURL(resp), "must not redirect to the OIDC end_session_endpoint after a password sign-in")
		})

		t.Run("OIDCIDTokenSurvives2FACompletion", func(t *testing.T) {
			// counterpart to PasswordSignInAfterOIDCClearsStaleIDTokenAndMethod: an OIDC sign-in
			// that required 2FA sets KeySignInMethod/KeyOIDCIDToken via handleTwoFactorRequired
			// *before* the 2FA step; handleSignIn (called on successful 2FA) must preserve them,
			// not just blindly clear on every sign-in completion
			mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockMemStore("dummy-sid-oidc-2fa-completion")}
			ctx, _ := contexttest.MockContext(t, "/user/oauth2/..../callback?code=dummy-code", mockOpt)
			handleTwoFactorRequired(ctx, oauthUser, false, map[string]any{
				session.KeySignInMethod: session.SignInMethodOAuth2,
				session.KeyOIDCIDToken:  "real-id-token-pending-2fa",
			})

			// simulates what TwoFactorPost does on a correct passcode
			ctx, _ = contexttest.MockContext(t, "/user/twofa", mockOpt)
			handleSignIn(ctx, oauthUser, false)
			assert.Equal(t, session.SignInMethodOAuth2, ctx.Session.Get(session.KeySignInMethod), "completing 2FA must not lose the OAuth2 sign-in method recorded before the 2FA step")
			assert.Equal(t, "real-id-token-pending-2fa", ctx.Session.Get(session.KeyOIDCIDToken), "completing 2FA must not lose the id_token recorded before the 2FA step")

			ctx, resp := contexttest.MockContext(t, "/user/logout", mockOpt)
			ctx.Doer = oauthUser
			SignOut(ctx)
			assert.Equal(t, http.StatusSeeOther, resp.Code)
			u, err := url.Parse(test.RedirectURL(resp))
			require.NoError(t, err)
			assert.Equal(t, "real-id-token-pending-2fa", u.Query().Get("id_token_hint"), "logout after 2FA completion must still include the id_token_hint from the original OIDC sign-in")
		})

		t.Run("PasswordSignInSkipsOIDC", func(t *testing.T) {
			// OAuth2-linked account signed in via password form must not hit end_session_endpoint.
			mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockMemStore("dummy-sid-password")}
			ctx, resp := contexttest.MockContext(t, "/user/logout", mockOpt)
			ctx.Doer = oauthUser
			SignOut(ctx)
			assert.Equal(t, http.StatusSeeOther, resp.Code)
			assert.Equal(t, "/", test.RedirectURL(resp))
		})
	})
}

func TestOpenIDRequireTwoFactor(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockMemStore("dummy-sid-openid")}

	user32 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 32}) // has a webauthn credential
	ctx, resp := contexttest.MockContext(t, "/user/openid/connect", mockOpt)
	require.NoError(t, ctx.Session.Set(session.KeySignInMethod, session.SignInMethodOAuth2))
	require.NoError(t, ctx.Session.Set(session.KeyOIDCIDToken, "stale-id-token-from-prior-oidc-login"))
	openIDRequireTwoFactor(ctx, user32, false, "https://example.com/id")
	assert.Equal(t, "/user/webauthn", test.RedirectURL(resp))
	unittest.AssertNotExistsBean(t, &user_model.UserOpenID{UID: user32.ID}) // not attached before the key answered
	assert.Empty(t, ctx.Session.Get(session.KeyOIDCIDToken), "legacy OpenID 2FA flow must not carry forward a stale id_token from an earlier OIDC session")
	assert.NotEqual(t, session.SignInMethodOAuth2, ctx.Session.Get(session.KeySignInMethod))

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	ctx, _ = contexttest.MockContext(t, "/user/openid/connect", mockOpt)
	openIDRequireTwoFactor(ctx, user2, false, "https://example.com/id")
	assert.False(t, ctx.Written())
}

func TestAutoSignInClearsStaleOIDCState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	nt, token, err := auth_service.CreateAuthTokenForUserID(t.Context(), user2.ID)
	require.NoError(t, err)

	mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockMemStore("dummy-sid-autologin")}
	ctx, _ := contexttest.MockContext(t, "/user/login", mockOpt)
	require.NoError(t, ctx.Session.Set(session.KeySignInMethod, session.SignInMethodOAuth2))
	require.NoError(t, ctx.Session.Set(session.KeyOIDCIDToken, "stale-id-token-from-prior-oidc-login"))
	ctx.Req.AddCookie(&http.Cookie{Name: setting.CookieRememberName, Value: nt.ID + ":" + token})

	succeeded, err := autoSignIn(ctx)
	require.NoError(t, err)
	require.True(t, succeeded)
	assert.Empty(t, ctx.Session.Get(session.KeyOIDCIDToken), "auto sign-in via remember-me cookie must not carry forward a stale id_token")
	assert.NotEqual(t, session.SignInMethodOAuth2, ctx.Session.Get(session.KeySignInMethod))
}
