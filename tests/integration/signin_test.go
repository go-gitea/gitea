// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/translation"
	"gitea.dev/modules/web"
	"gitea.dev/modules/web/middleware"
	"gitea.dev/routers"
	"gitea.dev/routers/web/auth"
	"gitea.dev/services/context"
	"gitea.dev/tests"

	"github.com/markbates/goth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLoginFailed(t *testing.T, username, password, message string) {
	session := emptyTestSession(t)
	req := NewRequestWithValues(t, "POST", "/user/login", map[string]string{
		"user_name": username,
		"password":  password,
	})
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	resultMsg := strings.TrimSpace(htmlDoc.doc.Find(".ui.message.flash-message").Text())
	assert.Equal(t, message, resultMsg)
}

func TestSignin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// add new user with user2's email
	user.Name = "testuser"
	user.LowerName = strings.ToLower(user.Name)
	user.ID = 0
	require.NoError(t, db.Insert(t.Context(), user))

	samples := []struct {
		username string
		password string
		message  string
	}{
		{username: "wrongUsername", password: "wrongPassword", message: translation.NewLocale("en-US").TrString("form.username_password_incorrect")},
		{username: "wrongUsername", password: "password", message: translation.NewLocale("en-US").TrString("form.username_password_incorrect")},
		{username: "user15", password: "wrongPassword", message: translation.NewLocale("en-US").TrString("form.username_password_incorrect")},
		{username: "user1@example.com", password: "wrongPassword", message: translation.NewLocale("en-US").TrString("form.username_password_incorrect")},
	}

	for _, s := range samples {
		testLoginFailed(t, s.username, s.password, s.message)
	}
}

func TestSigninWithRememberMe(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	baseURL, _ := url.Parse(setting.AppURL)

	for _, tc := range []struct {
		mode, remember, checkbox string
		remembered               bool
	}{
		{mode: "checked", remember: "on", checkbox: "input[name=remember][checked]", remembered: true},
		{mode: "unchecked", remember: "", checkbox: "input[name=remember]:not([checked])", remembered: false},
		{mode: "never", remember: "on", remembered: false},
		{mode: "always", remember: "", remembered: true},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			defer test.MockVariableValue(&setting.LogInRememberMe, tc.mode)()
			doc := NewHTMLParser(t, MakeRequest(t, NewRequest(t, "GET", "/user/login"), http.StatusOK).Body)
			AssertHTMLElement(t, doc, "input[name=remember]", tc.checkbox != "")
			if tc.checkbox != "" {
				AssertHTMLElement(t, doc, tc.checkbox, true)
				resp := MakeRequest(t, NewRequestWithValues(t, "POST", "/user/login", map[string]string{"user_name": user.Name, "password": "wrong"}), http.StatusOK)
				AssertHTMLElement(t, NewHTMLParser(t, resp.Body), "input[name=remember][checked]", false)
			}

			session := emptyTestSession(t)
			session.MakeRequest(t, NewRequestWithValues(t, "POST", "/user/login", map[string]string{
				"user_name": user.Name,
				"password":  userPassword,
				"remember":  tc.remember,
			}), http.StatusSeeOther)
			c := session.GetRawCookie(setting.CookieRememberName)
			require.Equal(t, tc.remembered, c != nil)
			if !tc.remembered {
				return
			}

			session = emptyTestSession(t)
			session.MakeRequest(t, NewRequest(t, "GET", "/user/settings"), http.StatusSeeOther)
			session.jar.SetCookies(baseURL, []*http.Cookie{c})
			session.MakeRequest(t, NewRequest(t, "GET", "/user/login"), http.StatusSeeOther)
			session.MakeRequest(t, NewRequest(t, "GET", "/user/settings"), http.StatusOK)
		})
	}
}

func TestEnablePasswordSignInFormAndEnablePasskeyAuth(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	mockLinkAccount := func(ctx *context.Context) {
		authSource := auth_model.Source{ID: 1}
		gothUser := goth.User{Email: "invalid-email", Name: "."}
		_ = auth.Oauth2SetLinkAccountData(ctx, auth.LinkAccountData{AuthSourceID: authSource.ID, GothUser: gothUser})
	}

	for _, enabled := range []bool{false, true} {
		t.Run("EnablePasswordSignInForm="+strconv.FormatBool(enabled), func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			defer test.MockVariableValue(&setting.Service.EnablePasswordSignInForm, enabled)()

			req := NewRequest(t, "GET", "/user/login")
			resp := MakeRequest(t, req, http.StatusOK)
			doc := NewHTMLParser(t, resp.Body)
			AssertHTMLElement(t, doc, "form[action='/user/login']", enabled)
			AssertHTMLElement(t, doc, "form:has(.signin-passkey) input[name=remember]", true)

			postStatus := http.StatusForbidden
			if enabled {
				postStatus = http.StatusOK
			}
			req = NewRequest(t, "POST", "/user/login")
			MakeRequest(t, req, postStatus)

			req = NewRequest(t, "GET", "/user/link_account")
			defer web.RouteMockReset()
			web.RouteMock(web.MockAfterMiddlewares, mockLinkAccount)
			resp = MakeRequest(t, req, http.StatusOK)
			doc = NewHTMLParser(t, resp.Body)
			AssertHTMLElement(t, doc, "form[action='/user/link_account_signin']", enabled)
		})
	}

	for _, enabled := range []bool{false, true} {
		t.Run("EnablePasskeyAuth="+strconv.FormatBool(enabled), func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			defer test.MockVariableValue(&setting.Service.EnablePasskeyAuth, enabled)()

			req := NewRequest(t, "GET", "/user/login")
			resp := MakeRequest(t, req, http.StatusOK)
			doc := NewHTMLParser(t, resp.Body)
			AssertHTMLElement(t, doc, ".signin-passkey", enabled)
		})
	}
}

func TestRequireSignInView(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("NoRequireSignInView", func(t *testing.T) {
		require.False(t, setting.Service.RequireSignInViewStrict)
		require.False(t, setting.Service.BlockAnonymousAccessExpensive)
		req := NewRequest(t, "GET", "/user2/repo1/src/branch/master")
		MakeRequest(t, req, http.StatusOK)
		req = NewRequest(t, "GET", "/-/ws")
		MakeRequest(t, req, http.StatusUpgradeRequired)
	})
	t.Run("RequireSignInView", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Service.RequireSignInViewStrict, true)()
		defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()
		t.Run("AccessPublicRepo", func(t *testing.T) {
			req := NewRequest(t, "GET", "/user2/repo1/src/branch/master")
			resp := MakeRequest(t, req, http.StatusSeeOther)
			assert.Equal(t, "/user/login?redirect_to=%2Fuser2%2Frepo1%2Fsrc%2Fbranch%2Fmaster", resp.Header().Get("Location"))
		})
		t.Run("UpdateTheme", func(t *testing.T) {
			session := emptyTestSession(t)
			req := NewRequest(t, "POST", "/-/web-theme/apply?theme=gitea-dark")
			session.MakeRequest(t, req, http.StatusOK)
			assert.Equal(t, "gitea-dark", session.GetSiteCookie(middleware.CookieTheme))
		})
	})
	t.Run("BlockAnonymousAccessExpensive", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Service.RequireSignInViewStrict, false)()
		defer test.MockVariableValue(&setting.Service.BlockAnonymousAccessExpensive, true)()
		defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

		req := NewRequest(t, "GET", "/user2/repo1")
		MakeRequest(t, req, http.StatusOK)
		req = NewRequest(t, "GET", "/-/ws")
		MakeRequest(t, req, http.StatusSeeOther)

		req = NewRequest(t, "GET", "/user2/repo1/src/branch/master")
		resp := MakeRequest(t, req, http.StatusSeeOther)
		assert.Equal(t, "/user/login?redirect_to=%2Fuser2%2Frepo1%2Fsrc%2Fbranch%2Fmaster", resp.Header().Get("Location"))
	})
}

func TestSignOut(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("NormalLogout", func(t *testing.T) {
		session := loginUser(t, "user2")

		req := NewRequest(t, "GET", "/user/logout")
		resp := session.MakeRequest(t, req, http.StatusSeeOther)
		assert.Equal(t, "/", resp.Header().Get("Location"))

		// logged out, try to view a private repo, should fail
		req = NewRequest(t, "GET", "/user2/repo2")
		session.MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("ReverseProxyLogoutRedirect", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Service.EnableReverseProxyAuth, true)()
		defer test.MockVariableValue(&setting.ReverseProxyLogoutRedirect, "/my-sso/logout?return_to=/my-sso/home")()

		session := loginUser(t, "user2")
		req := NewRequest(t, "GET", "/user/logout")
		resp := session.MakeRequest(t, req, http.StatusSeeOther)
		assert.Equal(t, "/my-sso/logout?return_to=/my-sso/home", resp.Header().Get("Location"))

		// logged out, try to view a private repo, should fail
		req = NewRequest(t, "GET", "/user2/repo2")
		session.MakeRequest(t, req, http.StatusNotFound)
	})
}
