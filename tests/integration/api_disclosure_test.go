// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	disclosure_model "gitea.dev/models/disclosure"
	api "gitea.dev/modules/structs"
	disclosure_service "gitea.dev/services/disclosure"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIDisclosures(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	owner, repo := setupDisclosureTestRepo(t, "disclosure-api", false)
	_, privateRepo := setupDisclosureTestRepo(t, "disclosure-api-private", true)
	ownerSession := loginUser(t, "user2") // owner of the test repos and their admin
	adminToken := getTokenForLoggedInUser(t, ownerSession, auth_model.AccessTokenScopeWriteRepository)
	otherSession := loginUser(t, "user4") // signed in, but not a maintainer of the test repos
	otherToken := getTokenForLoggedInUser(t, otherSession, auth_model.AccessTokenScopeWriteRepository)

	baseURL := fmt.Sprintf("/api/v1/repos/%s/%s/security/disclosures", repo.OwnerName, repo.Name)
	privateBaseURL := fmt.Sprintf("/api/v1/repos/%s/%s/security/disclosures", privateRepo.OwnerName, privateRepo.Name)

	published := createTestDisclosure(t, repo, owner, &disclosure_service.Input{
		Title:            "Buffer overflow in URL parser",
		Summary:          "A crafted URL can overflow a stack buffer in the URL parser",
		Content:          "## Description\n\nA crafted URL can overflow a stack buffer.",
		CVSSVector:       "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
		CweIds:           []string{"CWE-120"},
		CveID:            "CVE-2026-0001",
		AffectedVersions: []string{">= 1.0.0, < 1.4.0", "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"},
		FixedVersions:    []string{"1.4.0"},
		Status:           disclosure_model.StatusPublished,
	})
	draft := createTestDisclosure(t, repo, owner, &disclosure_service.Input{
		Title:            "TLS certificate verification bypass",
		Severity:         disclosure_model.SeverityHigh,
		AffectedVersions: []string{">= 2.0.0"},
	})
	privateDisclosure := createTestDisclosure(t, privateRepo, owner, &disclosure_service.Input{
		Title:    "Private repo disclosure",
		Severity: disclosure_model.SeverityLow,
		Status:   disclosure_model.StatusPublished,
	})

	t.Run("ListAnonymousSeesPublishedOnly", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", baseURL)
		resp := MakeRequest(t, req, http.StatusOK)
		var disclosures []*api.SecurityDisclosure
		DecodeJSON(t, resp, &disclosures)
		require.Len(t, disclosures, 1)
		d := disclosures[0]
		assert.Equal(t, published.ID, d.ID)
		assert.Equal(t, "SD-0001", d.Identifier)
		assert.Equal(t, "published", d.Status)
		assert.Equal(t, "critical", d.Severity)
		assert.Equal(t, []string{">= 1.0.0, < 1.4.0", "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"}, d.AffectedVersions)
		assert.Equal(t, []string{"1.4.0"}, d.FixedVersions)
		assert.Equal(t, "CVE-2026-0001", d.CveID)
		assert.Equal(t, []string{"CWE-120"}, d.CweIds)
		require.NotNil(t, d.CVSS)
		assert.Equal(t, "3.1", d.CVSS.Version)
		assert.InDelta(t, 9.8, d.CVSS.Score, 0.001)
		require.NotNil(t, d.PublishedAt)
		require.NotNil(t, d.Author)
		assert.Equal(t, "user2", d.Author.UserName)
		assert.True(t, strings.HasSuffix(d.URL, baseURL+"/"+fmt.Sprint(published.ID)), d.URL)
		assert.Equal(t, repo.Link()+fmt.Sprintf("/security/disclosures/%d", published.ID), d.HTMLURL)
	})

	t.Run("ListFilters", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		listIDs := func(url, token string) []int64 {
			req := NewRequest(t, "GET", url)
			if token != "" {
				req.AddTokenAuth(token)
			}
			resp := MakeRequest(t, req, http.StatusOK)
			var disclosures []*api.SecurityDisclosure
			DecodeJSON(t, resp, &disclosures)
			ids := make([]int64, 0, len(disclosures))
			for _, d := range disclosures {
				ids = append(ids, d.ID)
			}
			return ids
		}
		publishedID := []int64{published.ID}
		assert.Equal(t, publishedID, listIDs(baseURL+"?q=buffer", ""))
		assert.Equal(t, publishedID, listIDs(baseURL+"?q=BUFFER", "")) // case-insensitive
		assert.Equal(t, publishedID, listIDs(baseURL+"?severity=critical", ""))
		assert.Empty(t, listIDs(baseURL+"?severity=high", "")) // draft hidden from anonymous
		assert.Equal(t, publishedID, listIDs(baseURL+"?status=published", ""))
		assert.Empty(t, listIDs(baseURL+"?status=draft", ""))                                                  // draft hidden from anonymous
		assert.Equal(t, publishedID, listIDs(baseURL+"?version=1.2.0", ""))                                    // semver range match
		assert.Empty(t, listIDs(baseURL+"?version=1.4.0", ""))                                                 // outside the range
		assert.Equal(t, publishedID, listIDs(baseURL+"?version=a94a8fe5ccb19ba61c4c0873d391e987982fbbd3", "")) // exact non-semver match
		// admin sees drafts too
		assert.Equal(t, []int64{draft.ID}, listIDs(baseURL+"?version=2.5.0", adminToken))
		assert.Equal(t, []int64{draft.ID}, listIDs(baseURL+"?status=draft", adminToken))
		assert.ElementsMatch(t, []int64{published.ID, draft.ID}, listIDs(baseURL+"?page=1&limit=50", adminToken))

		// a private repo's disclosures are not accessible anonymously
		MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("%s/%d", privateBaseURL, privateDisclosure.ID)), http.StatusNotFound)
	})

	t.Run("GetDraftHiddenFromNonMaintainers", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("%s/%d", baseURL, draft.ID)), http.StatusNotFound)                          // anonymous
		MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("%s/%d", baseURL, draft.ID)).AddTokenAuth(otherToken), http.StatusNotFound) // non-admin
		resp := MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("%s/%d", baseURL, draft.ID)).AddTokenAuth(adminToken), http.StatusOK)
		var d api.SecurityDisclosure
		DecodeJSON(t, resp, &d)
		assert.Equal(t, "draft", d.Status)
		MakeRequest(t, NewRequest(t, "GET", baseURL+"/999").AddTokenAuth(adminToken), http.StatusNotFound)
	})

	t.Run("CreateForbiddenForNonMaintainer", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequestWithJSON(t, "POST", baseURL, &api.CreateSecurityDisclosureOption{
			Title: "x", Severity: "low",
		}).AddTokenAuth(otherToken)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("CreateValidation", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		post := func(opt *api.CreateSecurityDisclosureOption) *httptest.ResponseRecorder {
			req := NewRequestWithJSON(t, "POST", baseURL, opt).AddTokenAuth(adminToken)
			return MakeRequest(t, req, http.StatusBadRequest)
		}
		post(&api.CreateSecurityDisclosureOption{Title: "x", Severity: "bogus"})
		post(&api.CreateSecurityDisclosureOption{Title: "x", Severity: "low", CVSSVector: "AV:N/AC:L"})
		post(&api.CreateSecurityDisclosureOption{Title: "x", Severity: "low", CVSSVector: "CVSS:3.1/AV:N"}) // incomplete vector
		post(&api.CreateSecurityDisclosureOption{Title: "x"})                                               // neither severity nor CVSS vector
		post(&api.CreateSecurityDisclosureOption{Title: "x", Severity: "low", AffectedVersions: []string{">= broken"}})
		post(&api.CreateSecurityDisclosureOption{Title: "x", Severity: "low", FixedVersions: []string{">= 1.0.0"}})
	})

	t.Run("Credits", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// create with one local-user credit and one external credit
		req := NewRequestWithJSON(t, "POST", baseURL, &api.CreateSecurityDisclosureOption{
			Title:    "Credited disclosure",
			Severity: "low",
			Credits: []api.SecurityDisclosureCreditInput{
				{Role: "reporter", UserName: "user4"},
				{Role: "finder", Name: "Jane Doe", URL: "https://jane.example.com"},
			},
		}).AddTokenAuth(adminToken)
		resp := MakeRequest(t, req, http.StatusCreated)
		var created api.SecurityDisclosure
		DecodeJSON(t, resp, &created)
		require.Len(t, created.Credits, 2)
		assert.Equal(t, api.SecurityDisclosureCreditRole("reporter"), created.Credits[0].Role)
		assert.Equal(t, "user4", created.Credits[0].Name)
		require.NotNil(t, created.Credits[0].User)
		assert.Equal(t, "user4", created.Credits[0].User.UserName)
		assert.Equal(t, api.SecurityDisclosureCreditRole("finder"), created.Credits[1].Role)
		assert.Equal(t, "Jane Doe", created.Credits[1].Name)
		assert.Equal(t, "https://jane.example.com", created.Credits[1].URL)
		assert.Nil(t, created.Credits[1].User)

		// unknown local user is rejected
		req = NewRequestWithJSON(t, "POST", baseURL, &api.CreateSecurityDisclosureOption{
			Title: "x", Severity: "low",
			Credits: []api.SecurityDisclosureCreditInput{{Role: "reporter", UserName: "nosuchuser"}},
		}).AddTokenAuth(adminToken)
		MakeRequest(t, req, http.StatusBadRequest)

		// PATCH without credits keeps them, PATCH with [] clears them
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("%s/%d", baseURL, created.ID), &api.EditSecurityDisclosureOption{
			Title: "Credited disclosure (edited)",
		})
		resp = MakeRequest(t, req.AddTokenAuth(adminToken), http.StatusOK)
		var edited api.SecurityDisclosure
		DecodeJSON(t, resp, &edited)
		assert.Len(t, edited.Credits, 2)

		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("%s/%d", baseURL, created.ID), &api.EditSecurityDisclosureOption{
			Credits: []api.SecurityDisclosureCreditInput{},
		})
		resp = MakeRequest(t, req.AddTokenAuth(adminToken), http.StatusOK)
		DecodeJSON(t, resp, &edited)
		assert.Empty(t, edited.Credits)
	})

	t.Run("CreateDerivesSeverityFromCVSS", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		// no severity given: it is derived from the CVSS base score (7.5 -> high)
		req := NewRequestWithJSON(t, "POST", baseURL, &api.CreateSecurityDisclosureOption{
			Title:      "CVSS-derived severity",
			CVSSVector: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:N/A:N",
		}).AddTokenAuth(adminToken)
		resp := MakeRequest(t, req, http.StatusCreated)
		var created api.SecurityDisclosure
		DecodeJSON(t, resp, &created)
		assert.Equal(t, "high", created.Severity)
		require.NotNil(t, created.CVSS)
		assert.Equal(t, "3.1", created.CVSS.Version)
		assert.InDelta(t, 7.5, created.CVSS.Score, 0.001)

		// a CVSS vector wins over a conflicting manual severity (9.8 -> critical, not low)
		req = NewRequestWithJSON(t, "POST", baseURL, &api.CreateSecurityDisclosureOption{
			Title:      "CVSS wins over severity",
			Severity:   "low",
			CVSSVector: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
		}).AddTokenAuth(adminToken)
		resp = MakeRequest(t, req, http.StatusCreated)
		DecodeJSON(t, resp, &created)
		assert.Equal(t, "critical", created.Severity)
		assert.InDelta(t, 9.8, created.CVSS.Score, 0.001)
	})

	t.Run("CreateEditPublishDeleteAsMaintainer", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// create as draft
		req := NewRequestWithJSON(t, "POST", baseURL, &api.CreateSecurityDisclosureOption{
			Title:            "API created disclosure",
			Summary:          "summary",
			Description:      "markdown **content**",
			Severity:         "high",
			CVSSVector:       "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:H/SI:H/SA:H",
			AffectedVersions: []string{">= 9.0.0, < 9.1.0"},
			FixedVersions:    []string{"9.1.0"},
			References:       []string{"https://example.com/ref"},
		}).AddTokenAuth(adminToken)
		resp := MakeRequest(t, req, http.StatusCreated)
		var created api.SecurityDisclosure
		DecodeJSON(t, resp, &created)
		assert.Equal(t, "draft", created.Status)
		assert.Regexp(t, `^SD-\d{4}$`, created.Identifier)
		assert.Nil(t, created.PublishedAt)
		require.NotNil(t, created.CVSS)
		assert.Equal(t, "4.0", created.CVSS.Version)

		// anonymous cannot see the new draft
		MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("%s/%d", baseURL, created.ID)), http.StatusNotFound)

		// publish via PATCH
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("%s/%d", baseURL, created.ID), &api.EditSecurityDisclosureOption{
			Status: "published",
		}).AddTokenAuth(adminToken)
		resp = MakeRequest(t, req, http.StatusOK)
		var publishedResp api.SecurityDisclosure
		DecodeJSON(t, resp, &publishedResp)
		assert.Equal(t, "published", publishedResp.Status)
		assert.NotNil(t, publishedResp.PublishedAt)

		// edit fields, clear fixed versions with []
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("%s/%d", baseURL, created.ID), &api.EditSecurityDisclosureOption{
			Title:         "API created disclosure (renamed)",
			FixedVersions: []string{},
		}).AddTokenAuth(adminToken)
		resp = MakeRequest(t, req, http.StatusOK)
		var edited api.SecurityDisclosure
		DecodeJSON(t, resp, &edited)
		assert.Equal(t, "API created disclosure (renamed)", edited.Title)
		assert.Empty(t, edited.FixedVersions)
		assert.Equal(t, []string{">= 9.0.0, < 9.1.0"}, edited.AffectedVersions) // kept

		// version filter now matches the new published disclosure
		req = NewRequest(t, "GET", baseURL+"?version=9.0.5")
		resp = MakeRequest(t, req, http.StatusOK)
		var list []*api.SecurityDisclosure
		DecodeJSON(t, resp, &list)
		require.Len(t, list, 1)
		assert.Equal(t, created.ID, list[0].ID)

		// withdrawing requires a reason
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("%s/%d", baseURL, created.ID), &api.EditSecurityDisclosureOption{
			Status: "withdrawn",
		}).AddTokenAuth(adminToken)
		MakeRequest(t, req, http.StatusBadRequest)

		// withdraw with a reason
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("%s/%d", baseURL, created.ID), &api.EditSecurityDisclosureOption{
			Status:         "withdrawn",
			WithdrawReason: "reported in error",
		}).AddTokenAuth(adminToken)
		resp = MakeRequest(t, req, http.StatusOK)
		var withdrawn api.SecurityDisclosure
		DecodeJSON(t, resp, &withdrawn)
		assert.Equal(t, "withdrawn", withdrawn.Status)
		assert.Equal(t, "reported in error", withdrawn.WithdrawReason)

		// non-drafts cannot be deleted
		req = NewRequest(t, "DELETE", fmt.Sprintf("%s/%d", baseURL, created.ID)).AddTokenAuth(adminToken)
		MakeRequest(t, req, http.StatusBadRequest)
		MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("%s/%d", baseURL, created.ID)).AddTokenAuth(adminToken), http.StatusOK)

		// but a draft can be deleted
		req = NewRequestWithJSON(t, "POST", baseURL, &api.CreateSecurityDisclosureOption{
			Title: "draft to delete", Severity: "informative",
		}).AddTokenAuth(adminToken)
		resp = MakeRequest(t, req, http.StatusCreated)
		var draftResp api.SecurityDisclosure
		DecodeJSON(t, resp, &draftResp)
		assert.Equal(t, "informative", draftResp.Severity)
		req = NewRequest(t, "DELETE", fmt.Sprintf("%s/%d", baseURL, draftResp.ID)).AddTokenAuth(adminToken)
		MakeRequest(t, req, http.StatusNoContent)
		MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("%s/%d", baseURL, draftResp.ID)).AddTokenAuth(adminToken), http.StatusNotFound)
	})
}
