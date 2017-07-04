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
	MakeRequest(t, req, http.StatusOK)
}

func TestRenameUsername(t *testing.T) {
	prepareTestEnv(t)

	session := loginUser(t, "user2")
	req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
		"_csrf": GetCSRF(t, session, "/user/settings"),
		"name":  "newUsername",
		"email": "user2@example.com",
	})
	session.MakeRequest(t, req, http.StatusFound)

	models.AssertExistsAndLoadBean(t, &models.User{Name: "newUsername"})
	models.AssertNotExistsBean(t, &models.User{Name: "user2"})
}

func TestRenameInvalidUsername(t *testing.T) {
	prepareTestEnv(t)

	invalidUsernames := []string{
		"%2f*",
		"%2f.",
		"%2f..",
		"%00",
		"thisHas ASpace",
	}

	session := loginUser(t, "user2")
	for _, invalidUsername := range invalidUsernames {
		t.Logf("Testing username %s", invalidUsername)

		req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
			"_csrf": GetCSRF(t, session, "/user/settings"),
			"name":  invalidUsername,
			"email": "user2@example.com",
		})
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Contains(t,
			htmlDoc.doc.Find(".ui.negative.message").Text(),
			i18n.Tr("en", "form.alpha_dash_dot_error"),
		)

		models.AssertNotExistsBean(t, &models.User{Name: invalidUsername})
	}
}

func TestRenameReservedUsername(t *testing.T) {
	prepareTestEnv(t)

	reservedUsernames := []string{
		"help",
		"user",
		"template",
	}

	session := loginUser(t, "user2")
	for _, reservedUsername := range reservedUsernames {
		t.Logf("Testing username %s", reservedUsername)
		req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
			"_csrf": GetCSRF(t, session, "/user/settings"),
			"name":  reservedUsername,
			"email": "user2@example.com",
		})
		resp := session.MakeRequest(t, req, http.StatusFound)

		req = NewRequest(t, "GET", RedirectURL(t, resp))
		resp = session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Contains(t,
			htmlDoc.doc.Find(".ui.negative.message").Text(),
			i18n.Tr("en", "user.newName_reserved"),
		)

		models.AssertNotExistsBean(t, &models.User{Name: reservedUsername})
	}
}
