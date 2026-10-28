// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestSettingShowUserEmailExplore(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.UI.ShowUserEmail, true)()

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
}

func TestSettingShowUserEmailProfile(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	showUserEmail := setting.UI.ShowUserEmail

	// user1: keep_email_private = false, user2: keep_email_private = true

	var session *TestSession
	for _, profile := range []struct {
		login, name string
		showEmail   bool
		visible     bool
	}{
		// user1 can see own visible email
		{"user1", "user1", true, true},
		// user1 can not see user2's hidden email
		// Should only contain if the user visits their own profile page
		{"", "user2", true, false},
		// user2 can see user1's visible email
		{"user2", "user1", true, true},
		// user2 can see own hidden email
		{"user2", "user2", true, true},
		// user1 can see own (now hidden) email
		{"user1", "user1", false, true},
	} {
		setting.UI.ShowUserEmail = profile.showEmail
		if profile.login != "" {
			session = loginUser(t, profile.login)
		}
		req := NewRequest(t, "GET", "/"+profile.name)
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		if profile.visible {
			assert.Contains(t, htmlDoc.doc.Find(".user.profile").Text(), profile.name+"@example.com")
		} else {
			assert.NotContains(t, htmlDoc.doc.Find(".user.profile").Text(), profile.name+"@example.com")
		}
	}

	setting.UI.ShowUserEmail = showUserEmail
}

func TestSettingLandingPage(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.LandingPageURL)()

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
}
