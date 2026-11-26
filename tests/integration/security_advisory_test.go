// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	advisory_model "gitea.dev/models/advisory"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityAdvisoryWeb(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	admin := loginUser(t, "user2")
	reporter := loginUser(t, "user4")
	outsider := loginUser(t, "user5")
	const advisoriesURL = "/user2/repo1/security/advisories"
	const summary = "Incorrect permissions in uploads"
	const description = "Uploaded files have incorrect permissions."
	const vector = "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"

	page := func(session *TestSession, url string) string {
		return session.MakeRequest(t, NewRequest(t, "GET", url), http.StatusOK).Body.String()
	}
	privateReporting := func() bool {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		return repo.MustGetUnit(t.Context(), unit.TypeSecurityAdvisories).SecurityAdvisoriesConfig().PrivateVulnerabilityReporting
	}
	saveUnits := func(values map[string]string) {
		values["action"] = "advanced"
		values["enable_code"] = "on"
		admin.MakeRequest(t, NewRequestWithValues(t, "POST", "/user2/repo1/settings", values), http.StatusOK)
	}

	saveUnits(map[string]string{"enable_security_advisories": "on", "private_vulnerability_reporting": "on"})
	assert.True(t, privateReporting())

	assert.Contains(t, page(reporter, advisoriesURL), advisoriesURL+"/report")
	assert.Contains(t, page(reporter, advisoriesURL+"/report"), "Select an ecosystem")
	resp := reporter.MakeRequest(t, NewRequestWithURLValues(t, "POST", advisoriesURL+"/report", url.Values{
		"summary":     {summary},
		"content":     {description},
		"severity":    {"cvss:3.1"},
		"cvss_vector": {vector},
		"ecosystem":   {"", "go"},
	}), http.StatusOK)
	advisory := unittest.AssertExistsAndLoadBean(t, &advisory_model.Advisory{RepoID: 1, ReporterID: 4})
	assert.Equal(t, advisory_model.StateTriage, advisory.State)
	assert.Equal(t, advisory_model.SeverityCritical, advisory.Severity)
	assert.Equal(t, vector, advisory.CvssVector)
	unittest.AssertCount(t, &advisory_model.Vulnerability{AdvisoryID: advisory.ID}, 1)
	unittest.AssertExistsAndLoadBean(t, &advisory_model.Vulnerability{AdvisoryID: advisory.ID, Ecosystem: "go"})
	advisoryURL := advisoriesURL + "/" + advisory.Identifier
	assert.Equal(t, advisoryURL, *test.ParseJSONRedirect(resp.Body.Bytes()).Redirect)

	assert.Contains(t, page(admin, advisoriesURL+"?state=triage&q=uploads"), advisory.Identifier)
	assert.NotContains(t, page(admin, advisoriesURL+"?state=triage&q=nomatch"), advisory.Identifier)

	outsider.MakeRequest(t, NewRequest(t, "GET", advisoryURL), http.StatusNotFound)
	MakeRequest(t, NewRequest(t, "GET", advisoryURL), http.StatusNotFound)
	assert.NotContains(t, page(outsider, advisoriesURL+"?state=triage"), advisory.Identifier)
	outsider.MakeRequest(t, NewRequestWithValues(t, "POST", advisoryURL+"/state", map[string]string{"state": "draft"}), http.StatusNotFound)

	labels := map[string]string{"id": "1,2"}
	reporter.MakeRequest(t, NewRequestWithValues(t, "POST", advisoryURL+"/labels", labels), http.StatusForbidden)
	resp = admin.MakeRequest(t, NewRequestWithValues(t, "POST", advisoryURL+"/labels", labels), http.StatusOK)
	assert.JSONEq(t, `{"ok":true}`, resp.Body.String())
	unittest.AssertExistsAndLoadBean(t, &advisory_model.LabelLink{AdvisoryID: advisory.ID, LabelID: 1})
	unittest.AssertExistsAndLoadBean(t, &advisory_model.LabelLink{AdvisoryID: advisory.ID, LabelID: 2})

	const comment = "Confirmed, the fix is in progress."
	reporter.MakeRequest(t, NewRequestWithValues(t, "POST", advisoryURL+"/comments", map[string]string{"content": comment}), http.StatusOK)
	body := page(admin, advisoryURL)
	assert.Contains(t, body, comment)
	assert.Contains(t, body, `data-update-url="`+advisoryURL+`/labels"`)
	assert.NotContains(t, body, "data-mentions-url")
	assert.Contains(t, body, `data-global-click="onConvContentQuoteReply"`)
	assert.NotContains(t, body, `data-global-init="initSecurityAdvisoryConfidentialQuote"`)
	assert.Contains(t, body, `data-global-click="onConvContentDelete" data-comment-hash-tag="advisory-comment-`)

	const confidentialComment = "The maintenance team is reviewing the release schedule."
	admin.MakeRequest(t, NewRequestWithValues(t, "POST", advisoryURL+"/comments", map[string]string{"content": confidentialComment, "is_confidential": "on"}), http.StatusOK)
	body = page(admin, advisoryURL)
	assert.Contains(t, body, confidentialComment)
	assert.Contains(t, body, `data-global-init="initSecurityAdvisoryConfidentialQuote"`)
	body = page(reporter, advisoryURL)
	assert.Contains(t, body, comment)
	assert.NotContains(t, body, confidentialComment)
	assert.Contains(t, body, `class="ui dropdown full-width disabled"`)
	assert.Contains(t, body, `data-global-click="onConvContentEdit"`)

	assert.Contains(t, page(admin, advisoriesURL+"/new"), `name="label_ids"`)
	outsider.MakeRequest(t, NewRequest(t, "GET", advisoriesURL+"/new"), http.StatusForbidden)
	body = page(reporter, advisoryURL+"/edit")
	assert.Contains(t, body, `name="severity" value="cvss:3.1"`)
	assert.Contains(t, body, vector)
	assert.NotContains(t, body, `name="credit_user"`)

	advisory = unittest.AssertExistsAndLoadBean(t, &advisory_model.Advisory{ID: advisory.ID})
	admin.MakeRequest(t, NewRequestWithValues(t, "POST", advisoryURL+"/edit", map[string]string{
		"summary":         summary,
		"content":         description,
		"content_version": strconv.Itoa(advisory.ContentVersion),
		"severity":        "medium",
		"cvss_vector":     vector,
		"ecosystem":       "go",
	}), http.StatusOK)
	advisory = unittest.AssertExistsAndLoadBean(t, &advisory_model.Advisory{ID: advisory.ID})
	assert.Equal(t, advisory_model.SeverityMedium, advisory.Severity)
	assert.Empty(t, advisory.CvssVector)
	unittest.AssertCount(t, &advisory_model.LabelLink{AdvisoryID: advisory.ID}, 2)

	reporter.MakeRequest(t, NewRequestWithValues(t, "POST", advisoryURL+"/state", map[string]string{"state": "draft"}), http.StatusForbidden)
	admin.MakeRequest(t, NewRequestWithValues(t, "POST", advisoryURL+"/state", map[string]string{"state": "draft"}), http.StatusOK)
	admin.MakeRequest(t, NewRequestWithValues(t, "POST", advisoryURL+"/state", map[string]string{"state": "published"}), http.StatusOK)

	body = page(nil, advisoryURL)
	assert.Contains(t, body, summary)
	assert.NotContains(t, body, comment)
	reporter.MakeRequest(t, NewRequest(t, "GET", advisoryURL+"/edit"), http.StatusForbidden)

	saveUnits(map[string]string{})
	admin.MakeRequest(t, NewRequest(t, "GET", advisoryURL), http.StatusNotFound)
	require.False(t, unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).UnitEnabled(t.Context(), unit.TypeSecurityAdvisories))
}
