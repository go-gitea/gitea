// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIDisclosures(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	ownerSession := loginUser(t, "user2") // user2 owns repo1 and is its admin
	adminToken := getTokenForLoggedInUser(t, ownerSession, auth_model.AccessTokenScopeWriteRepository)
	otherSession := loginUser(t, "user4") // signed in, but not a maintainer of repo1
	otherToken := getTokenForLoggedInUser(t, otherSession, auth_model.AccessTokenScopeWriteRepository)

	baseURL := fmt.Sprintf("/api/v1/repos/%s/%s/security/disclosures", repo.OwnerName, repo.Name)

	t.Run("ListAnonymousSeesPublishedOnly", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", baseURL)
		resp := MakeRequest(t, req, http.StatusOK)
		var disclosures []*api.SecurityDisclosure
		DecodeJSON(t, resp, &disclosures)
		require.Len(t, disclosures, 1)
		d := disclosures[0]
		assert.Equal(t, int64(1), d.ID)
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
		assert.Equal(t, []int64{1}, listIDs(baseURL+"?q=buffer", ""))
		assert.Equal(t, []int64{1}, listIDs(baseURL+"?q=BUFFER", "")) // case-insensitive
		assert.Equal(t, []int64{1}, listIDs(baseURL+"?severity=critical", ""))
		assert.Empty(t, listIDs(baseURL+"?severity=high", "")) // draft hidden from anonymous
		assert.Equal(t, []int64{1}, listIDs(baseURL+"?status=published", ""))
		assert.Empty(t, listIDs(baseURL+"?status=draft", ""))                                                 // draft hidden from anonymous
		assert.Equal(t, []int64{1}, listIDs(baseURL+"?version=1.2.0", ""))                                    // semver range match
		assert.Empty(t, listIDs(baseURL+"?version=1.4.0", ""))                                                // outside the range
		assert.Equal(t, []int64{1}, listIDs(baseURL+"?version=a94a8fe5ccb19ba61c4c0873d391e987982fbbd3", "")) // exact non-semver match
		// admin sees drafts too
		assert.Equal(t, []int64{2}, listIDs(baseURL+"?version=2.5.0", adminToken))
		assert.Equal(t, []int64{2}, listIDs(baseURL+"?status=draft", adminToken))
		assert.ElementsMatch(t, []int64{1, 2}, listIDs(baseURL+"?page=1&limit=50", adminToken))
	})

	t.Run("GetDraftHiddenFromNonMaintainers", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		MakeRequest(t, NewRequest(t, "GET", baseURL+"/2"), http.StatusNotFound)                          // anonymous
		MakeRequest(t, NewRequest(t, "GET", baseURL+"/2").AddTokenAuth(otherToken), http.StatusNotFound) // non-admin
		resp := MakeRequest(t, NewRequest(t, "GET", baseURL+"/2").AddTokenAuth(adminToken), http.StatusOK)
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
		assert.Equal(t, "SD-0005", created.Identifier) // two disclosures were created by the CVSS derivation subtest above
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
		var published api.SecurityDisclosure
		DecodeJSON(t, resp, &published)
		assert.Equal(t, "published", published.Status)
		assert.NotNil(t, published.PublishedAt)

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
		var draft api.SecurityDisclosure
		DecodeJSON(t, resp, &draft)
		assert.Equal(t, "informative", draft.Severity)
		req = NewRequest(t, "DELETE", fmt.Sprintf("%s/%d", baseURL, draft.ID)).AddTokenAuth(adminToken)
		MakeRequest(t, req, http.StatusNoContent)
		MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("%s/%d", baseURL, draft.ID)).AddTokenAuth(adminToken), http.StatusNotFound)
	})
}
