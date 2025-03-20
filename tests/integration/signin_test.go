// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/tests"

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
	resultMsg := htmlDoc.doc.Find(".ui.message>p").Text()

	assert.EqualValues(t, message, resultMsg)
}

func TestSignin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// add new user with user2's email
	user.Name = "testuser"
	user.LowerName = strings.ToLower(user.Name)
	user.ID = 0
	require.NoError(t, db.Insert(db.DefaultContext, user))

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

	session := emptyTestSession(t)
	req := NewRequestWithValues(t, "POST", "/user/login", map[string]string{
		"user_name": user.Name,
		"password":  userPassword,
		"remember":  "on",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	c := session.GetRawCookie(setting.CookieRememberName)
	assert.NotNil(t, c)

	session = emptyTestSession(t)

	// Without session the settings page should not be reachable
	req = NewRequest(t, "GET", "/user/settings")
	session.MakeRequest(t, req, http.StatusSeeOther)

	req = NewRequest(t, "GET", "/user/login")
	// Set the remember me cookie for the login GET request
	session.jar.SetCookies(baseURL, []*http.Cookie{c})
	session.MakeRequest(t, req, http.StatusSeeOther)

	// With session the settings page should be reachable
	req = NewRequest(t, "GET", "/user/settings")
	session.MakeRequest(t, req, http.StatusOK)
}

func TestEnablePasswordSignInFormAndEnablePasskeyAuth(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	mockLinkAccount := func(ctx *context.Context) {
		gothUser := goth.User{Email: "invalid-email", Name: "."}
		_ = ctx.Session.Set("linkAccountGothUser", gothUser)
	}

	t.Run("EnablePasswordSignInForm=false", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.Service.EnablePasswordSignInForm, false)()

		req := NewRequest(t, "GET", "/user/login")
		resp := MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)
		AssertHTMLElement(t, doc, "form[action='/user/login']", false)

		req = NewRequest(t, "POST", "/user/login")
		MakeRequest(t, req, http.StatusForbidden)

		req = NewRequest(t, "GET", "/user/link_account")
		defer web.RouteMockReset()
		web.RouteMock(web.MockAfterMiddlewares, mockLinkAccount)
		resp = MakeRequest(t, req, http.StatusOK)
		doc = NewHTMLParser(t, resp.Body)
		AssertHTMLElement(t, doc, "form[action='/user/link_account_signin']", false)
	})

	t.Run("EnablePasswordSignInForm=true", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.Service.EnablePasswordSignInForm, true)()

		req := NewRequest(t, "GET", "/user/login")
		resp := MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)
		AssertHTMLElement(t, doc, "form[action='/user/login']", true)

		req = NewRequest(t, "POST", "/user/login")
		MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", "/user/link_account")
		defer web.RouteMockReset()
		web.RouteMock(web.MockAfterMiddlewares, mockLinkAccount)
		resp = MakeRequest(t, req, http.StatusOK)
		doc = NewHTMLParser(t, resp.Body)
		AssertHTMLElement(t, doc, "form[action='/user/link_account_signin']", true)
	})

	t.Run("EnablePasskeyAuth=false", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.Service.EnablePasskeyAuth, false)()

		req := NewRequest(t, "GET", "/user/login")
		resp := MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)
		AssertHTMLElement(t, doc, ".signin-passkey", false)
	})

	t.Run("EnablePasskeyAuth=true", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.Service.EnablePasskeyAuth, true)()

		req := NewRequest(t, "GET", "/user/login")
		resp := MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)
		AssertHTMLElement(t, doc, ".signin-passkey", true)
	})
}
