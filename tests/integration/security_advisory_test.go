// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
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

	resp := reporter.MakeRequest(t, NewRequest(t, "GET", advisoriesURL), http.StatusOK)
	assert.Contains(t, resp.Body.String(), advisoriesURL+"/report")
	reporter.MakeRequest(t, NewRequest(t, "GET", advisoriesURL+"/report"), http.StatusOK)
	resp = reporter.MakeRequest(t, NewRequestWithValues(t, "POST", advisoriesURL+"/report", map[string]string{
		"summary":                  "Path traversal in uploads",
		"content":                  "Uploading `../x` writes outside the data directory.",
		"cvss_v3_vector":           "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
		"ecosystem":                "go",
		"package_name":             "gitea.dev",
		"vulnerable_version_range": "< 1.0.0",
	}), http.StatusOK)
	a := unittest.AssertExistsAndLoadBean(t, &advisory_model.Advisory{RepoID: 1, ReporterID: 4})
	assert.Equal(t, advisory_model.StateTriage, a.State)
	assert.Equal(t, advisory_model.SeverityCritical, a.Severity)
	advisoryURL := advisoriesURL + "/" + a.Identifier
	assert.Equal(t, advisoryURL, *test.ParseJSONRedirect(resp.Body.Bytes()).Redirect)

	resp = admin.MakeRequest(t, NewRequest(t, "GET", advisoriesURL+"?state=triage&q=traversal"), http.StatusOK)
	assert.Contains(t, resp.Body.String(), a.Identifier)
	resp = admin.MakeRequest(t, NewRequest(t, "GET", advisoriesURL+"?state=triage&q=nomatch"), http.StatusOK)
	assert.NotContains(t, resp.Body.String(), a.Identifier)

	reporter.MakeRequest(t, NewRequest(t, "GET", advisoryURL), http.StatusOK)
	outsider.MakeRequest(t, NewRequest(t, "GET", advisoryURL), http.StatusNotFound)
	MakeRequest(t, NewRequest(t, "GET", advisoryURL), http.StatusNotFound)
	resp = outsider.MakeRequest(t, NewRequest(t, "GET", advisoriesURL+"?state=triage"), http.StatusOK)
	assert.NotContains(t, resp.Body.String(), a.Identifier)
	outsider.MakeRequest(t, NewRequestWithValues(t, "POST", advisoryURL+"/state", map[string]string{"state": "draft"}), http.StatusNotFound)

	const comment = "Confirmed, the fix is in progress."
	reporter.MakeRequest(t, NewRequestWithValues(t, "POST", advisoryURL+"/comments", map[string]string{"content": comment}), http.StatusOK)
	resp = admin.MakeRequest(t, NewRequest(t, "GET", advisoryURL), http.StatusOK)
	assert.Contains(t, resp.Body.String(), comment)
	assert.NotContains(t, resp.Body.String(), "data-mentions-url", "mentions would suggest users who cannot see the advisory")

	const internalComment = "The reporter asked for a bounty twice."
	admin.MakeRequest(t, NewRequestWithValues(t, "POST", advisoryURL+"/comments", map[string]string{"content": internalComment, "is_internal": "on"}), http.StatusOK)
	resp = admin.MakeRequest(t, NewRequest(t, "GET", advisoryURL), http.StatusOK)
	assert.Contains(t, resp.Body.String(), internalComment)
	resp = reporter.MakeRequest(t, NewRequest(t, "GET", advisoryURL), http.StatusOK)
	assert.Contains(t, resp.Body.String(), comment)
	assert.NotContains(t, resp.Body.String(), internalComment)

	admin.MakeRequest(t, NewRequest(t, "GET", advisoriesURL+"/new"), http.StatusOK)
	outsider.MakeRequest(t, NewRequest(t, "GET", advisoriesURL+"/new"), http.StatusForbidden)
	resp = reporter.MakeRequest(t, NewRequest(t, "GET", advisoryURL+"/edit"), http.StatusOK)
	assert.Contains(t, resp.Body.String(), "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H")
	assert.NotContains(t, resp.Body.String(), `name="credit_user"`, "only admins manage credits")

	reporter.MakeRequest(t, NewRequestWithValues(t, "POST", advisoryURL+"/state", map[string]string{"state": "draft"}), http.StatusForbidden)
	admin.MakeRequest(t, NewRequestWithValues(t, "POST", advisoryURL+"/state", map[string]string{"state": "draft"}), http.StatusOK)
	admin.MakeRequest(t, NewRequestWithValues(t, "POST", advisoryURL+"/state", map[string]string{"state": "published"}), http.StatusOK)

	resp = MakeRequest(t, NewRequest(t, "GET", advisoryURL), http.StatusOK)
	assert.Contains(t, resp.Body.String(), "Path traversal in uploads")
	assert.NotContains(t, resp.Body.String(), comment, "the discussion stays private after publishing")
	reporter.MakeRequest(t, NewRequest(t, "GET", advisoryURL+"/edit"), http.StatusForbidden)

	saveUnits(map[string]string{})
	admin.MakeRequest(t, NewRequest(t, "GET", advisoryURL), http.StatusNotFound)
	require.False(t, unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).UnitEnabled(t.Context(), unit.TypeSecurityAdvisories))
}
