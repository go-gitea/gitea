// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
	"github.com/unknwon/i18n"
)

func testLoginFailed(t *testing.T, username, password, message string) {
	session := emptyTestSession(t)
	req := NewRequestWithValues(t, "POST", "/user/login", map[string]string{
		"_csrf":     GetCSRF(t, session, "/user/login"),
		"user_name": username,
		"password":  password,
	})
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	resultMsg := htmlDoc.doc.Find(".ui.message>p").Text()

	assert.EqualValues(t, message, resultMsg)
}

func TestSignin(t *testing.T) {
	defer prepareTestEnv(t)()

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)

	// add new user with user2's email
	user.Name = "testuser"
	user.LowerName = strings.ToLower(user.Name)
	user.ID = 0
	models.AssertSuccessfulInsert(t, user)

	samples := []struct {
		username string
		password string
		message  string
	}{
		{username: "wrongUsername", password: "wrongPassword", message: i18n.Tr("en", "form.username_password_incorrect")},
		{username: "wrongUsername", password: "password", message: i18n.Tr("en", "form.username_password_incorrect")},
		{username: "user15", password: "wrongPassword", message: i18n.Tr("en", "form.username_password_incorrect")},
		{username: "user1@example.com", password: "wrongPassword", message: i18n.Tr("en", "form.username_password_incorrect")},
		// test for duplicate email
		{username: "user2@example.com", password: "password", message: i18n.Tr("en", "form.email_been_used")},
	}

	for _, s := range samples {
		testLoginFailed(t, s.username, s.password, s.message)
	}
}
