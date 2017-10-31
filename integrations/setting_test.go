// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestSettingShowUserEmailExplore(t *testing.T) {
	prepareTestEnv(t)

	showUserEmail := setting.UI.ShowUserEmail
	setting.UI.ShowUserEmail = true

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/explore/users")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.Contains(t,
		htmlDoc.doc.Find(".ui.user.list").Text(),
		"user2@example.com",
	)

	setting.UI.ShowUserEmail = false

	req = NewRequest(t, "GET", "/explore/users")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.NotContains(t,
		htmlDoc.doc.Find(".ui.user.list").Text(),
		"user2@example.com",
	)

	setting.UI.ShowUserEmail = showUserEmail
}

func TestSettingShowUserEmailProfile(t *testing.T) {
	prepareTestEnv(t)

	showUserEmail := setting.UI.ShowUserEmail
	setting.UI.ShowUserEmail = true

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.Contains(t,
		htmlDoc.doc.Find(".user.profile").Text(),
		"user2@example.com",
	)

	setting.UI.ShowUserEmail = false

	req = NewRequest(t, "GET", "/user2")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.NotContains(t,
		htmlDoc.doc.Find(".user.profile").Text(),
		"user2@example.com",
	)

	setting.UI.ShowUserEmail = showUserEmail
}
