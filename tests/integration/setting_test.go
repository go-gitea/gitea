// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestSettingShowUserEmailExplore(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	showUserEmail := setting.UI.ShowUserEmail
	setting.UI.ShowUserEmail = true

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/explore/users?sort=alphabetically")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.Contains(t,
		htmlDoc.doc.Find(".explore.users").Text(),
		"user34@example.com",
	)

	setting.UI.ShowUserEmail = false

	req = NewRequest(t, "GET", "/explore/users?sort=alphabetically")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.NotContains(t,
		htmlDoc.doc.Find(".explore.users").Text(),
		"user34@example.com",
	)

	setting.UI.ShowUserEmail = showUserEmail
}

func TestSettingShowUserEmailProfile(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// note: in this scenario, user2's email will always be private
	// this setting only controls the email visibility of user1
	// user1: keep_email_private = false, user2: keep_email_private = true
	showUserEmail := setting.UI.ShowUserEmail

	// user1 can see own visible email
	setting.UI.ShowUserEmail = true
	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/user1")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.NotContains(t, htmlDoc.doc.Find(".user.profile").Text(), "user1@noreply.example.org")
	assert.Contains(t, htmlDoc.doc.Find(".user.profile").Text(), "user1@example.com")

	// user1 can not see user2's placeholder / hidden, primary email
	req = NewRequest(t, "GET", "/user2")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.NotContains(t, htmlDoc.doc.Find(".user.profile").Text(), "user2@noreply.example.org")
	assert.NotContains(t, htmlDoc.doc.Find(".user.profile").Text(), "user2@example.com")

	// user2 can see user1's visible email
	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/user1")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.NotContains(t, htmlDoc.doc.Find(".user.profile").Text(), "user1@noreply.example.org")
	assert.Contains(t, htmlDoc.doc.Find(".user.profile").Text(), "user1@example.com")

	// user2's profile only shows a placeholder email
	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/user2")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.Contains(t, htmlDoc.doc.Find(".user.profile").Text(), "user2@noreply.example.org")
	assert.NotContains(t, htmlDoc.doc.Find(".user.profile").Text(), "user2@example.com")

	// now that user1's email is hidden, user1 should only be
	// able to see the placeholder email on their profile
	setting.UI.ShowUserEmail = false
	session = loginUser(t, "user1")
	req = NewRequest(t, "GET", "/user1")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.NotContains(t, htmlDoc.doc.Find(".user.profile").Text(), "user1@noreply.example.org")
	assert.Contains(t, htmlDoc.doc.Find(".user.profile").Text(), "user1@example.com")

	setting.UI.ShowUserEmail = showUserEmail
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
