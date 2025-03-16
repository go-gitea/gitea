// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/setting/config"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestSettingShowUserEmailExplore(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	showUserEmail := setting.Config().UI.ShowUserEmail.Value(t.Context())
	showUserEmailValue := config.Value[bool]{}
	setting.Config().UI.ShowUserEmail = showUserEmailValue.WithDefault(true)
	config.GetDynGetter().InvalidateCache()

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/explore/users?sort=alphabetically")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.Contains(t,
		htmlDoc.doc.Find(".explore.users").Text(),
		"user34@example.com",
	)

	showUserEmailValue = config.Value[bool]{}
	setting.Config().UI.ShowUserEmail = showUserEmailValue.WithDefault(false)
	config.GetDynGetter().InvalidateCache()

	req = NewRequest(t, "GET", "/explore/users?sort=alphabetically")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.NotContains(t,
		htmlDoc.doc.Find(".explore.users").Text(),
		"user34@example.com",
	)

	showUserEmailValue = config.Value[bool]{}
	setting.Config().UI.ShowUserEmail = showUserEmailValue.WithDefault(showUserEmail)
	config.GetDynGetter().InvalidateCache()
}

func TestSettingShowUserEmailProfile(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	showUserEmail := setting.Config().UI.ShowUserEmail.Value(t.Context())

	// user1: keep_email_private = false, user2: keep_email_private = true
	showUserEmailValue := config.Value[bool]{}
	setting.Config().UI.ShowUserEmail = showUserEmailValue.WithDefault(true)
	config.GetDynGetter().InvalidateCache()

	// user1 can see own visible email
	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/user1")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.Contains(t, htmlDoc.doc.Find(".user.profile").Text(), "user1@example.com")

	// user1 can not see user2's hidden email
	req = NewRequest(t, "GET", "/user2")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	// Should only contain if the user visits their own profile page
	assert.NotContains(t, htmlDoc.doc.Find(".user.profile").Text(), "user2@example.com")

	// user2 can see user1's visible email
	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/user1")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.Contains(t, htmlDoc.doc.Find(".user.profile").Text(), "user1@example.com")

	// user2 can see own hidden email
	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/user2")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.Contains(t, htmlDoc.doc.Find(".user.profile").Text(), "user2@example.com")

	showUserEmailValue = config.Value[bool]{}
	setting.Config().UI.ShowUserEmail = showUserEmailValue.WithDefault(false)
	config.GetDynGetter().InvalidateCache()

	// user1 can see own (now hidden) email
	session = loginUser(t, "user1")
	req = NewRequest(t, "GET", "/user1")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.Contains(t, htmlDoc.doc.Find(".user.profile").Text(), "user1@example.com")

	showUserEmailValue = config.Value[bool]{}
	setting.Config().UI.ShowUserEmail = showUserEmailValue.WithDefault(showUserEmail)
	config.GetDynGetter().InvalidateCache()
}

func TestSettingLandingPage(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	landingPage := setting.LandingPageURL

	setting.LandingPageURL = setting.LandingPageHome
	req := NewRequest(t, "GET", "/")
	MakeRequest(t, req, http.StatusOK)

	setting.LandingPageURL = setting.LandingPageExplore
	req = NewRequest(t, "GET", "/")
	resp := MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "/explore", resp.Header().Get("Location"))

	setting.LandingPageURL = setting.LandingPageOrganizations
	req = NewRequest(t, "GET", "/")
	resp = MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "/explore/organizations", resp.Header().Get("Location"))

	setting.LandingPageURL = setting.LandingPageLogin
	req = NewRequest(t, "GET", "/")
	resp = MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "/user/login", resp.Header().Get("Location"))

	setting.LandingPageURL = landingPage
}
