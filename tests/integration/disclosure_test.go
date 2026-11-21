// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestDisclosureWeb(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("ListAnonymousSeesPublishedOnly", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", "/user2/repo1/security")
		resp := MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		body := htmlDoc.Find(".page-content").Text()
		assert.Contains(t, body, "Buffer overflow in URL parser")
		assert.NotContains(t, body, "TLS certificate verification bypass") // draft is hidden
	})

	t.Run("ViewPublishedAnonymous", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", "/user2/repo1/security/1")
		resp := MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		body := htmlDoc.Find(".page-content").Text()
		assert.Contains(t, body, "SD-0001")
		assert.Contains(t, body, "Buffer overflow in URL parser")
		assert.Contains(t, body, "A crafted URL can overflow a stack buffer")
		assert.Contains(t, body, ">= 1.0.0, < 1.4.0")
	})

	t.Run("ViewDraftHiddenFromAnonymousAndNonMaintainer", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/security/2"), http.StatusNotFound)
		session := loginUser(t, "user4") // signed in, but not a maintainer of repo1
		session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/security/2"), http.StatusNotFound)
	})

	t.Run("NewPageRequiresMaintainer", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/security/new"), http.StatusSeeOther) // anonymous is redirected to sign in
		session := loginUser(t, "user4")
		session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/security/new"), http.StatusNotFound) // non-maintainer
		owner := loginUser(t, "user2")
		owner.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/security/new"), http.StatusOK)
	})

	t.Run("CreatePublishAndDeleteAsMaintainer", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		owner := loginUser(t, "user2")

		// create as draft
		req := NewRequestWithValues(t, "POST", "/user2/repo1/security/new", map[string]string{
			"title":             "Web created disclosure",
			"summary":           "web summary",
			"content":           "web **content**",
			"severity":          "moderate",
			"affected_versions": ">= 5.0.0, < 5.2.0",
			"fixed_versions":    "5.2.0",
		})
		resp := owner.MakeRequest(t, req, http.StatusSeeOther)
		location := resp.Header().Get("Location")
		assert.Contains(t, location, "/user2/repo1/security/")

		// anonymous cannot see the new draft
		MakeRequest(t, NewRequest(t, "GET", location), http.StatusNotFound)

		// publish it
		owner.MakeRequest(t, NewRequest(t, "POST", location+"/publish"), http.StatusSeeOther)

		// now anonymous can see it
		resp = MakeRequest(t, NewRequest(t, "GET", location), http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Contains(t, htmlDoc.Find(".page-content").Text(), "Web created disclosure")

		// filter by the affected version
		resp = MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/security?version=5.1.0"), http.StatusOK)
		assert.Contains(t, NewHTMLParser(t, resp.Body).Find(".page-content").Text(), "Web created disclosure")
		resp = MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/security?version=5.2.0"), http.StatusOK)
		assert.NotContains(t, NewHTMLParser(t, resp.Body).Find(".page-content").Text(), "Web created disclosure")

		// delete it
		owner.MakeRequest(t, NewRequest(t, "POST", location+"/delete"), http.StatusSeeOther)
		MakeRequest(t, NewRequest(t, "GET", location), http.StatusNotFound)
	})
}
