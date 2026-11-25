// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	disclosure_model "gitea.dev/models/disclosure"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	disclosure_service "gitea.dev/services/disclosure"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupDisclosureTestRepo creates a fresh repository owned by user2; new repos get the security unit by default
func setupDisclosureTestRepo(t *testing.T, name string, isPrivate bool) (*user_model.User, *repo_model.Repository) {
	t.Helper()
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo, err := repo_service.CreateRepository(t.Context(), owner, owner, repo_service.CreateRepoOptions{
		Name:          name,
		AutoInit:      false,
		DefaultBranch: "master",
		IsPrivate:     isPrivate,
	})
	require.NoError(t, err)
	return owner, repo
}

func createTestDisclosure(t *testing.T, repo *repo_model.Repository, doer *user_model.User, in *disclosure_service.Input) *disclosure_model.Disclosure {
	t.Helper()
	d, err := disclosure_service.CreateDisclosure(t.Context(), repo, doer, in)
	require.NoError(t, err)
	return d
}

// publishedDisclosureInput is the standard published disclosure used by the web and API tests
func publishedDisclosureInput() *disclosure_service.Input {
	return &disclosure_service.Input{
		Title:            "Buffer overflow in URL parser",
		Summary:          "A crafted URL can overflow a stack buffer in the URL parser",
		Content:          "## Description\n\nA crafted URL can overflow a stack buffer.\n\n## Impact\n\nRemote code execution.",
		CVSSVector:       "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
		CweIds:           []string{"CWE-120"},
		CveID:            "CVE-2026-0001",
		References:       []string{"https://example.com/advisories/1"},
		AffectedVersions: []string{">= 1.0.0, < 1.4.0", "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"},
		FixedVersions:    []string{"1.4.0"},
		Status:           disclosure_model.StatusPublished,
	}
}

// draftDisclosureInput is the standard draft disclosure used by the web and API tests
func draftDisclosureInput() *disclosure_service.Input {
	return &disclosure_service.Input{
		Title:            "TLS certificate verification bypass",
		Summary:          "Certificate hostname checks can be skipped for crafted certificates",
		Content:          "Draft content about the TLS bypass.",
		Severity:         disclosure_model.SeverityHigh,
		AffectedVersions: []string{">= 2.0.0"},
	}
}

func TestDisclosureWeb(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	owner, repo := setupDisclosureTestRepo(t, "disclosure-web", false)
	_, privateRepo := setupDisclosureTestRepo(t, "disclosure-private", true)
	disclosuresURL := repo.Link() + "/security/disclosures"

	published := createTestDisclosure(t, repo, owner, publishedDisclosureInput())
	draft := createTestDisclosure(t, repo, owner, draftDisclosureInput())
	createTestDisclosure(t, privateRepo, owner, &disclosure_service.Input{
		Title:    "Private repo disclosure",
		Summary:  "A disclosure in another repo",
		Content:  "content",
		Severity: disclosure_model.SeverityLow,
		Status:   disclosure_model.StatusPublished,
	})

	t.Run("SecurityHome", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", repo.Link()+"/security")
		resp := MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		body := htmlDoc.Find(".page-content").Text()
		assert.Contains(t, body, "Security disclosures")
		assert.Equal(t, 1, htmlDoc.Find(fmt.Sprintf(`a[href="%s"]`, disclosuresURL)).Length())
	})

	t.Run("ListAnonymousSeesPublishedOnly", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", disclosuresURL)
		resp := MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		body := htmlDoc.Find(".page-content").Text()
		assert.Contains(t, body, "Buffer overflow in URL parser")
		assert.NotContains(t, body, "TLS certificate verification bypass") // draft is hidden
	})

	t.Run("Feed", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// the list page advertises the feed via link tags and a button
		resp := MakeRequest(t, NewRequest(t, "GET", disclosuresURL), http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Equal(t, 1, htmlDoc.Find(fmt.Sprintf(`link[rel="alternate"][type="application/rss+xml"][href="%s.rss"]`, disclosuresURL)).Length())
		assert.Equal(t, 1, htmlDoc.Find(fmt.Sprintf(`link[rel="alternate"][type="application/atom+xml"][href="%s.atom"]`, disclosuresURL)).Length())
		assert.Equal(t, 1, htmlDoc.Find(fmt.Sprintf(`a[href="%s.rss"]`, disclosuresURL)).Length())

		// anonymous readers of the public repo get the feed, drafts are never included
		resp = MakeRequest(t, NewRequest(t, "GET", disclosuresURL+".rss"), http.StatusOK)
		assert.Equal(t, "application/rss+xml; charset=utf-8", resp.Header().Get("Content-Type"))
		body := resp.Body.String()
		assert.Contains(t, body, "Buffer overflow in URL parser")
		assert.Contains(t, body, "SD-0001")
		assert.NotContains(t, body, "TLS certificate verification bypass") // draft
		assert.NotContains(t, body, "Private repo disclosure")             // other repo

		resp = MakeRequest(t, NewRequest(t, "GET", disclosuresURL+".atom"), http.StatusOK)
		assert.Equal(t, "application/atom+xml; charset=utf-8", resp.Header().Get("Content-Type"))
		assert.Contains(t, resp.Body.String(), "Buffer overflow in URL parser")

		// the feed of a private repo is not accessible anonymously
		MakeRequest(t, NewRequest(t, "GET", privateRepo.Link()+"/security/disclosures.rss"), http.StatusNotFound)
	})

	t.Run("ViewPublishedAnonymous", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", fmt.Sprintf("%s/%d", disclosuresURL, published.ID))
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
		draftURL := fmt.Sprintf("%s/%d", disclosuresURL, draft.ID)
		MakeRequest(t, NewRequest(t, "GET", draftURL), http.StatusNotFound)
		session := loginUser(t, "user4") // signed in, but not a maintainer of the repo
		session.MakeRequest(t, NewRequest(t, "GET", draftURL), http.StatusNotFound)
	})

	t.Run("NewPageRequiresMaintainer", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		MakeRequest(t, NewRequest(t, "GET", disclosuresURL+"/new"), http.StatusSeeOther) // anonymous is redirected to sign in
		session := loginUser(t, "user4")
		session.MakeRequest(t, NewRequest(t, "GET", disclosuresURL+"/new"), http.StatusNotFound) // non-maintainer
		owner := loginUser(t, "user2")
		resp := owner.MakeRequest(t, NewRequest(t, "GET", disclosuresURL+"/new"), http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Equal(t, 1, htmlDoc.Find(`input[name="cvss_vector"]`).Length())
		assert.Equal(t, 1, htmlDoc.Find(`select[name="severity"]`).Length())
	})

	t.Run("CreatePublishAndDeleteAsMaintainer", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		owner := loginUser(t, "user2")

		// create as draft
		req := NewRequestWithValues(t, "POST", disclosuresURL+"/new", map[string]string{
			"title":             "Web created disclosure",
			"summary":           "web summary",
			"content":           "web **content**",
			"severity":          "moderate",
			"affected_versions": ">= 5.0.0, < 5.2.0",
			"fixed_versions":    "5.2.0",
		})
		resp := owner.MakeRequest(t, req, http.StatusSeeOther)
		location := resp.Header().Get("Location")
		assert.Contains(t, location, disclosuresURL+"/")

		// anonymous cannot see the new draft
		MakeRequest(t, NewRequest(t, "GET", location), http.StatusNotFound)

		// publish it
		owner.MakeRequest(t, NewRequest(t, "POST", location+"/publish"), http.StatusSeeOther)

		// now anonymous can see it
		resp = MakeRequest(t, NewRequest(t, "GET", location), http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Contains(t, htmlDoc.Find(".page-content").Text(), "Web created disclosure")

		// filter by the affected version
		resp = MakeRequest(t, NewRequest(t, "GET", disclosuresURL+"?version=5.1.0"), http.StatusOK)
		assert.Contains(t, NewHTMLParser(t, resp.Body).Find(".page-content").Text(), "Web created disclosure")
		resp = MakeRequest(t, NewRequest(t, "GET", disclosuresURL+"?version=5.2.0"), http.StatusOK)
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
		req = NewRequestWithValues(t, "POST", disclosuresURL+"/new", map[string]string{
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
		req := NewRequestWithValues(t, "POST", disclosuresURL+"/new", map[string]string{
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
		req = NewRequestWithValues(t, "POST", disclosuresURL+"/new", map[string]string{
			"title": "bad vector", "severity": "low", "cvss_vector": "CVSS:3.1/AV:N",
		})
		owner.MakeRequest(t, req, http.StatusOK) // re-renders the form with an error flash

		// neither severity nor vector is rejected too
		req = NewRequestWithValues(t, "POST", disclosuresURL+"/new", map[string]string{
			"title": "no severity at all",
		})
		owner.MakeRequest(t, req, http.StatusOK)
	})

	t.Run("CreateWithCredits", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		owner := loginUser(t, "user2")

		// one local user credit and one external person credit
		req := NewRequestWithValues(t, "POST", disclosuresURL+"/new", map[string]string{
			"title":         "Credited disclosure",
			"severity":      "low",
			"credit_role_0": "reporter",
			"credit_who_0":  "user4",
			"credit_role_1": "finder",
			"credit_who_1":  "Jane Doe",
			"credit_type_1": "external",
			"credit_url_1":  "https://jane.example.com",
		})
		resp := owner.MakeRequest(t, req, http.StatusSeeOther)
		location := resp.Header().Get("Location")

		resp = owner.MakeRequest(t, NewRequest(t, "GET", location), http.StatusOK)
		body := NewHTMLParser(t, resp.Body).Find(".page-content").Text()
		assert.Contains(t, body, "user4")
		assert.Contains(t, body, "Jane Doe")
		assert.Contains(t, body, "Reporter")
		assert.Contains(t, body, "Finder")

		// the edit form is prefilled with the credits
		resp = owner.MakeRequest(t, NewRequest(t, "GET", location+"/edit"), http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Equal(t, "user4", htmlDoc.Find(`input[name="credit_who_0"]`).AttrOr("value", ""))
		assert.Equal(t, "reporter", htmlDoc.Find(`select[name="credit_role_0"] option[selected]`).AttrOr("value", ""))
		assert.Equal(t, "Jane Doe", htmlDoc.Find(`input[name="credit_who_1"]`).AttrOr("value", ""))
		assert.Equal(t, "https://jane.example.com", htmlDoc.Find(`input[name="credit_url_1"]`).AttrOr("value", ""))
		assert.Equal(t, "external", htmlDoc.Find(`select[name="credit_type_1"] option[selected]`).AttrOr("value", ""))

		// the credits editor is a collapsible section in the main column, not the sidebar
		assert.Equal(t, 1, htmlDoc.Find(`.issue-content-left details.disclosure-credits[open]`).Length())
		assert.Equal(t, 0, htmlDoc.Find(`.issue-content-right #disclosure-credits-field`).Length())
		assert.Equal(t, 1, htmlDoc.Find(`#disclosure-credit-add`).Length())
		assert.Equal(t, 1, htmlDoc.Find(`#disclosure-credit-row-template`).Length())

		// an unknown local user is rejected
		req = NewRequestWithValues(t, "POST", disclosuresURL+"/new", map[string]string{
			"title": "bad credit", "severity": "low",
			"credit_role_0": "reporter", "credit_who_0": "nosuchuser",
		})
		owner.MakeRequest(t, req, http.StatusOK) // re-renders the form with an error

		// an invalid external URL is rejected
		req = NewRequestWithValues(t, "POST", disclosuresURL+"/new", map[string]string{
			"title": "bad credit url", "severity": "low",
			"credit_role_0": "finder", "credit_who_0": "Jane Doe", "credit_type_0": "external", "credit_url_0": "notaurl",
		})
		owner.MakeRequest(t, req, http.StatusOK)
	})
}
