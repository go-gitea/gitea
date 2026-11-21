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

	t.Run("SecurityHome", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", "/user2/repo1/security")
		resp := MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		body := htmlDoc.Find(".page-content").Text()
		assert.Contains(t, body, "Security disclosures")
		assert.Equal(t, 1, htmlDoc.Find(`a[href="/user2/repo1/security/disclosures"]`).Length())
	})

	t.Run("ListAnonymousSeesPublishedOnly", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", "/user2/repo1/security/disclosures")
		resp := MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		body := htmlDoc.Find(".page-content").Text()
		assert.Contains(t, body, "Buffer overflow in URL parser")
		assert.NotContains(t, body, "TLS certificate verification bypass") // draft is hidden
	})

	t.Run("ViewPublishedAnonymous", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", "/user2/repo1/security/disclosures/1")
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
		MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/security/disclosures/2"), http.StatusNotFound)
		session := loginUser(t, "user4") // signed in, but not a maintainer of repo1
		session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/security/disclosures/2"), http.StatusNotFound)
	})

	t.Run("NewPageRequiresMaintainer", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/security/disclosures/new"), http.StatusSeeOther) // anonymous is redirected to sign in
		session := loginUser(t, "user4")
		session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/security/disclosures/new"), http.StatusNotFound) // non-maintainer
		owner := loginUser(t, "user2")
		resp := owner.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/security/disclosures/new"), http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Equal(t, 1, htmlDoc.Find(`input[name="cvss_vector"]`).Length())
		assert.Equal(t, 1, htmlDoc.Find(`select[name="severity"]`).Length())
	})

	t.Run("CreatePublishAndDeleteAsMaintainer", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		owner := loginUser(t, "user2")

		// create as draft
		req := NewRequestWithValues(t, "POST", "/user2/repo1/security/disclosures/new", map[string]string{
			"title":             "Web created disclosure",
			"summary":           "web summary",
			"content":           "web **content**",
			"severity":          "moderate",
			"affected_versions": ">= 5.0.0, < 5.2.0",
			"fixed_versions":    "5.2.0",
		})
		resp := owner.MakeRequest(t, req, http.StatusSeeOther)
		location := resp.Header().Get("Location")
		assert.Contains(t, location, "/user2/repo1/security/disclosures/")

		// anonymous cannot see the new draft
		MakeRequest(t, NewRequest(t, "GET", location), http.StatusNotFound)

		// publish it
		owner.MakeRequest(t, NewRequest(t, "POST", location+"/publish"), http.StatusSeeOther)

		// now anonymous can see it
		resp = MakeRequest(t, NewRequest(t, "GET", location), http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Contains(t, htmlDoc.Find(".page-content").Text(), "Web created disclosure")

		// filter by the affected version
		resp = MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/security/disclosures?version=5.1.0"), http.StatusOK)
		assert.Contains(t, NewHTMLParser(t, resp.Body).Find(".page-content").Text(), "Web created disclosure")
		resp = MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/security/disclosures?version=5.2.0"), http.StatusOK)
		assert.NotContains(t, NewHTMLParser(t, resp.Body).Find(".page-content").Text(), "Web created disclosure")

		// withdrawing requires a reason
		owner.MakeRequest(t, NewRequestWithValues(t, "POST", location+"/withdraw", map[string]string{}), http.StatusSeeOther)
		resp = MakeRequest(t, NewRequest(t, "GET", location), http.StatusOK)
		assert.Contains(t, NewHTMLParser(t, resp.Body).Find(".page-content").Text(), "Web created disclosure") // still published

		// withdraw with a reason
		owner.MakeRequest(t, NewRequestWithValues(t, "POST", location+"/withdraw", map[string]string{"reason": "reported in error"}), http.StatusSeeOther)
		resp = MakeRequest(t, NewRequest(t, "GET", location), http.StatusOK)
		body := NewHTMLParser(t, resp.Body).Find(".page-content").Text()
		assert.Contains(t, body, "Withdrawn")
		assert.Contains(t, body, "reported in error")

		// published/withdrawn disclosures cannot be deleted
		owner.MakeRequest(t, NewRequest(t, "POST", location+"/delete"), http.StatusSeeOther)
		MakeRequest(t, NewRequest(t, "GET", location), http.StatusOK) // still there

		// a draft can be deleted
		req = NewRequestWithValues(t, "POST", "/user2/repo1/security/disclosures/new", map[string]string{
			"title": "Draft to delete", "severity": "low",
		})
		resp = owner.MakeRequest(t, req, http.StatusSeeOther)
		draftLocation := resp.Header().Get("Location")
		owner.MakeRequest(t, NewRequest(t, "POST", draftLocation+"/delete"), http.StatusSeeOther)
		MakeRequest(t, NewRequest(t, "GET", draftLocation), http.StatusNotFound)
	})

	t.Run("CreateDerivesSeverityFromCVSS", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		owner := loginUser(t, "user2")

		// no severity submitted: the severity is derived server-side from the CVSS base score
		req := NewRequestWithValues(t, "POST", "/user2/repo1/security/disclosures/new", map[string]string{
			"title":       "CVSS-derived web disclosure",
			"cvss_vector": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:N/A:N",
		})
		resp := owner.MakeRequest(t, req, http.StatusSeeOther)
		location := resp.Header().Get("Location")

		resp = owner.MakeRequest(t, NewRequest(t, "GET", location), http.StatusOK) // drafts need the maintainer session
		body := NewHTMLParser(t, resp.Body).Find(".page-content").Text()
		assert.Contains(t, body, "High") // 7.5 derives to high
		assert.Contains(t, body, "7.5")

		// an incomplete vector is rejected
		req = NewRequestWithValues(t, "POST", "/user2/repo1/security/disclosures/new", map[string]string{
			"title": "bad vector", "severity": "low", "cvss_vector": "CVSS:3.1/AV:N",
		})
		owner.MakeRequest(t, req, http.StatusOK) // re-renders the form with an error flash

		// neither severity nor vector is rejected too
		req = NewRequestWithValues(t, "POST", "/user2/repo1/security/disclosures/new", map[string]string{
			"title": "no severity at all",
		})
		owner.MakeRequest(t, req, http.StatusOK)
	})
}
