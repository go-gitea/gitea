// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/Unknwon/i18n"
	"github.com/stretchr/testify/assert"
)

func TestViewUser(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user2")
	resp := MakeRequest(req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
}

func TestRenameInvalidUsername(t *testing.T) {
	prepareTestEnv(t)

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user/settings")
	resp := session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	htmlDoc := NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"name":  "%2f*", // not valid
	})
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.Contains(t,
		htmlDoc.doc.Find(".ui.negative.message").Text(),
		i18n.Tr("en", "form.alpha_dash_dot_error"),
	)

	models.AssertNotExistsBean(t, &models.User{Name: "%2f*"})
}
