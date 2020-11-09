// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"github.com/stretchr/testify/assert"
	"github.com/unknwon/i18n"
)

func TestSignup(t *testing.T) {
	defer prepareTestEnv(t)()

	setting.Service.EnableCaptcha = false

	req := NewRequestWithValues(t, "POST", "/user/sign_up", map[string]string{
		"user_name": "exampleUser",
		"email":     "exampleUser@example.com",
		"password":  "examplePassword!1",
		"retype":    "examplePassword!1",
	})
	MakeRequest(t, req, http.StatusFound)

	// should be able to view new user's page
	req = NewRequest(t, "GET", "/exampleUser")
	MakeRequest(t, req, http.StatusOK)
}

func TestSignupInvalidEmail(t *testing.T) {
	defer prepareTestEnv(t)()

	setting.Service.EnableCaptcha = false

	req := NewRequestWithValues(t, "POST", "/user/sign_up", map[string]string{
		"user_name": "exampleUser",
		"email":     "exampleUser@example.com\r\n",
		"password":  "examplePassword!1",
		"retype":    "examplePassword!1",
	})
	resp := MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.Equal(t,
		i18n.Tr("en", "form.email_invalid", nil),
		strings.TrimSpace(htmlDoc.doc.Find(".ui.message").Text()),
	)
}
