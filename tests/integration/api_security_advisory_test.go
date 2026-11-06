// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	advisory_model "gitea.dev/models/advisory"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	webhook_model "gitea.dev/models/webhook"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPISecurityAdvisory(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	writer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})
	require.NoError(t, repo_service.AddOrUpdateCollaborator(t.Context(), repo, writer, perm.AccessModeWrite))

	adminToken := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	adminPublicOnlyToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)
	reporterToken := getUserToken(t, "user4", auth_model.AccessTokenScopeWriteRepository)
	outsiderToken := getUserToken(t, "user5", auth_model.AccessTokenScopeWriteRepository)
	writerToken := getUserToken(t, writer.Name, auth_model.AccessTokenScopeWriteRepository)
	blockedToken := getUserToken(t, "user29", auth_model.AccessTokenScopeWriteRepository)

	const baseURL = "/api/v1/repos/user2/repo1/security-advisories"
	const pvrURL = "/api/v1/repos/user2/repo1/private-vulnerability-reporting"
	report := &api.CreatePrivateVulnerabilityReportOption{
		Summary:          "XSS in the markdown renderer",
		Description:      "Rendering `<img onerror>` executes scripts.",
		CVSSVectorString: "CVSS:3.1/AV:N/AC:L/PR:N/UI:R/S:C/C:L/I:L/A:N",
		Vulnerabilities:  []*api.RepositoryAdvisoryVulnerability{{Package: &api.RepositoryAdvisoryPackage{Ecosystem: "go", Name: "gitea.dev"}, VulnerableVersionRange: "< 1.2.3"}},
	}

	advisoryRequest(t, "GET", baseURL, adminToken, nil, http.StatusNotFound) // the unit is disabled, also for admins
	require.NoError(t, repo_service.UpdateRepositoryUnits(t.Context(), repo, []repo_model.RepoUnit{{RepoID: repo.ID, Type: unit.TypeSecurityAdvisories}}, nil))

	assert.False(t, advisoryJSON(t, "GET", pvrURL, "", nil, http.StatusOK, &api.PrivateVulnerabilityReporting{}).Enabled)
	advisoryRequest(t, "POST", baseURL+"/reports", reporterToken, report, http.StatusForbidden)
	advisoryRequest(t, "PUT", pvrURL, outsiderToken, nil, http.StatusForbidden)
	advisoryRequest(t, "PUT", pvrURL, adminToken, nil, http.StatusNoContent)
	// enabling the already enabled unit must keep its config
	advisoryRequest(t, "PATCH", "/api/v1/repos/user2/repo1", adminToken, &api.EditRepoOption{HasSecurityAdvisories: new(true)}, http.StatusOK)
	assert.True(t, advisoryJSON(t, "GET", pvrURL, "", nil, http.StatusOK, &api.PrivateVulnerabilityReporting{}).Enabled)

	// webhooks sending everything must not forward undisclosed reports
	everythingHook := createTestWebhook(t, repo.ID, &webhook_module.HookEvent{SendEverything: true})
	reportHook := createTestWebhook(t, repo.ID, &webhook_module.HookEvent{ChooseEvents: true, HookEvents: webhook_module.HookEvents{webhook_module.HookEventRepositoryAdvisoryReported: true}})

	advisoryRequest(t, "POST", baseURL+"/reports", blockedToken, report, http.StatusForbidden)
	reported := advisoryJSON(t, "POST", baseURL+"/reports", reporterToken, report, http.StatusCreated, &api.RepositoryAdvisory{})
	assert.Equal(t, "triage", reported.State)
	assert.Equal(t, "medium", *reported.Severity)
	assert.Equal(t, "user4", reported.Author.UserName)
	assert.False(t, reported.Submission.Accepted)
	advisoryURL := baseURL + "/" + reported.Identifier
	unittest.AssertCount(t, &webhook_model.HookTask{HookID: everythingHook.ID}, 0)
	unittest.AssertExistsAndLoadBean(t, &webhook_model.HookTask{HookID: reportHook.ID, EventType: webhook_module.HookEventRepositoryAdvisoryReported})

	t.Run("DraftVisibility", func(t *testing.T) {
		advisoryRequest(t, "GET", advisoryURL, writerToken, nil, http.StatusNotFound)
		advisoryRequest(t, "GET", advisoryURL, adminPublicOnlyToken, nil, http.StatusNotFound)
		advisoryRequest(t, "GET", advisoryURL, "", nil, http.StatusNotFound)
		advisoryRequest(t, "PATCH", advisoryURL, outsiderToken, map[string]any{"summary": "x"}, http.StatusNotFound)
		assert.Empty(t, advisoryJSON(t, "GET", baseURL, adminPublicOnlyToken, nil, http.StatusOK, []*api.RepositoryAdvisory{}))
	})

	t.Run("Edit", func(t *testing.T) {
		// participants can edit the content, but not what only maintainers decide
		advisoryRequest(t, "PATCH", advisoryURL, reporterToken, map[string]any{"state": "draft"}, http.StatusForbidden)
		advisoryRequest(t, "PATCH", advisoryURL, reporterToken, map[string]any{"cve_id": "CVE-2026-1234"}, http.StatusForbidden)
		advisoryRequest(t, "PATCH", advisoryURL, reporterToken, map[string]any{"cwe_ids": []string{"CWE-79"}}, http.StatusOK)
		assert.Equal(t, []string{"CWE-79"}, advisoryJSON(t, "GET", advisoryURL, reporterToken, nil, http.StatusOK, &api.RepositoryAdvisory{}).CweIDs)

		advisoryRequest(t, "PATCH", advisoryURL, adminToken, map[string]any{"severity": "high"}, http.StatusUnprocessableEntity)
		advisoryRequest(t, "PATCH", advisoryURL, adminToken, map[string]any{"state": "published"}, http.StatusUnprocessableEntity)
		// an invalid collaborator rejects the whole request before anything is saved
		advisoryRequest(t, "PATCH", advisoryURL, adminToken, map[string]any{"summary": "changed", "collaborating_users": []string{"user29"}}, http.StatusForbidden)
		edited := advisoryJSON(t, "PATCH", advisoryURL, adminToken, map[string]any{
			"state":               "draft",
			"description":         "rewritten for publishing",
			"cvss_vector_string":  "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N",
			"collaborating_users": []string{"user5"},
		}, http.StatusOK, &api.RepositoryAdvisory{})
		assert.Equal(t, report.Summary, edited.Summary)
		assert.Equal(t, "draft", edited.State)
		assert.Equal(t, "critical", *edited.Severity)
		assert.NotNil(t, edited.CVSSSeverities.CvssV3, "the 3.1 vector is kept")
		require.Len(t, edited.CollaboratingUsers, 1)
		assert.True(t, edited.Submission.Accepted)
		accepted := unittest.AssertExistsAndLoadBean(t, &advisory_model.Advisory{RepoID: repo.ID, Identifier: reported.Identifier})
		assert.Equal(t, "rewritten for publishing", accepted.Description)
		assert.Equal(t, report.Description, accepted.ReportDescription, "the report is kept as it was accepted")

		advisoryRequest(t, "GET", advisoryURL, outsiderToken, nil, http.StatusOK)
		advisoryRequest(t, "PATCH", advisoryURL, adminToken, map[string]any{"collaborating_users": []string{}}, http.StatusOK)
		advisoryRequest(t, "GET", advisoryURL, outsiderToken, nil, http.StatusNotFound)
	})

	t.Run("Comments", func(t *testing.T) {
		commentsURL := advisoryURL + "/comments"
		postComment := func(token string, opt *api.RepositoryAdvisoryCommentOption) string {
			return fmt.Sprintf("%s/%d", commentsURL, advisoryJSON(t, "POST", commentsURL, token, opt, http.StatusCreated, &api.RepositoryAdvisoryComment{}).ID)
		}
		advisoryRequest(t, "GET", commentsURL, writerToken, nil, http.StatusNotFound)
		reporterCommentURL := postComment(reporterToken, &api.RepositoryAdvisoryCommentOption{Body: "steps attached"})
		adminCommentURL := postComment(adminToken, &api.RepositoryAdvisoryCommentOption{Body: "confirmed"})

		advisoryRequest(t, "PATCH", adminCommentURL, reporterToken, &api.RepositoryAdvisoryCommentOption{Body: "x"}, http.StatusForbidden)
		advisoryRequest(t, "DELETE", adminCommentURL, reporterToken, nil, http.StatusForbidden)
		assert.Equal(t, "steps updated", advisoryJSON(t, "PATCH", reporterCommentURL, reporterToken, &api.RepositoryAdvisoryCommentOption{Body: "steps updated"}, http.StatusOK, &api.RepositoryAdvisoryComment{}).Body)
		advisoryRequest(t, "DELETE", reporterCommentURL, adminToken, nil, http.StatusNoContent)

		advisoryRequest(t, "POST", commentsURL, reporterToken, &api.RepositoryAdvisoryCommentOption{Body: "x", IsInternal: true}, http.StatusForbidden)
		internalCommentURL := postComment(adminToken, &api.RepositoryAdvisoryCommentOption{Body: "the reporter seems to sell this", IsInternal: true})
		advisoryRequest(t, "PATCH", internalCommentURL, reporterToken, &api.RepositoryAdvisoryCommentOption{Body: "x"}, http.StatusNotFound)

		comments := advisoryJSON(t, "GET", commentsURL, reporterToken, nil, http.StatusOK, []*api.RepositoryAdvisoryComment{})
		require.Len(t, comments, 1)
		assert.Equal(t, "user2", comments[0].User.UserName)
		comments = advisoryJSON(t, "GET", commentsURL, adminToken, nil, http.StatusOK, []*api.RepositoryAdvisoryComment{})
		require.Len(t, comments, 2)
		assert.True(t, comments[1].IsInternal)
	})

	t.Run("Publish", func(t *testing.T) {
		assert.Equal(t, "user2", advisoryJSON(t, "PATCH", advisoryURL, adminToken, map[string]any{"state": "published"}, http.StatusOK, &api.RepositoryAdvisory{}).Publisher.UserName)
		public := advisoryJSON(t, "GET", advisoryURL, outsiderToken, nil, http.StatusOK, &api.RepositoryAdvisory{})
		assert.Nil(t, public.Author)
		assert.Nil(t, public.CollaboratingUsers)
		assert.Nil(t, public.Submission)
		advisoryRequest(t, "GET", advisoryURL, adminPublicOnlyToken, nil, http.StatusOK)
		advisoryRequest(t, "PATCH", advisoryURL, reporterToken, map[string]any{"summary": "x"}, http.StatusForbidden)

		// the published payload is sent without the private parts
		task := unittest.AssertExistsAndLoadBean(t, &webhook_model.HookTask{HookID: everythingHook.ID, EventType: webhook_module.HookEventRepositoryAdvisory})
		assert.Contains(t, task.PayloadContent, `"action": "published"`)
		assert.Contains(t, task.PayloadContent, `"author": null`)
	})

	t.Run("CloseAsDuplicate", func(t *testing.T) {
		original := advisoryJSON(t, "POST", baseURL, adminToken, &api.CreateRepositoryAdvisoryOption{Summary: "Original", Description: "details"}, http.StatusCreated, &api.RepositoryAdvisory{})
		originalURL := baseURL + "/" + original.Identifier
		duplicateURL := baseURL + "/" + advisoryJSON(t, "POST", baseURL+"/reports", outsiderToken, report, http.StatusCreated, &api.RepositoryAdvisory{}).Identifier
		closeAsDuplicate := map[string]any{"state": "closed", "close_reason": "duplicate", "duplicate_of": original.Identifier}
		advisoryRequest(t, "PATCH", duplicateURL, adminToken, map[string]any{"state": "closed"}, http.StatusUnprocessableEntity)
		advisoryRequest(t, "PATCH", duplicateURL, adminToken, map[string]any{"state": "closed", "close_reason": "duplicate"}, http.StatusUnprocessableEntity)
		closed := advisoryJSON(t, "PATCH", duplicateURL, adminToken, closeAsDuplicate, http.StatusOK, &api.RepositoryAdvisory{})
		assert.Equal(t, "duplicate", closed.CloseReason)
		assert.Equal(t, original.Identifier, closed.DuplicateOf)

		// the reporter of the duplicate can follow the original, but not edit it
		advisoryRequest(t, "GET", originalURL+"/comments", outsiderToken, nil, http.StatusOK)
		advisoryRequest(t, "PATCH", originalURL, outsiderToken, map[string]any{"summary": "x"}, http.StatusForbidden)
		advisoryRequest(t, "PATCH", duplicateURL, adminToken, map[string]any{"state": "draft"}, http.StatusOK)
		advisoryRequest(t, "GET", originalURL+"/comments", outsiderToken, nil, http.StatusNotFound) // reopening revokes the access
		advisoryRequest(t, "PATCH", duplicateURL, adminToken, closeAsDuplicate, http.StatusOK)
		assert.Len(t, advisoryJSON(t, "GET", baseURL+"?close_reason=duplicate", adminToken, nil, http.StatusOK, []*api.RepositoryAdvisory{}), 1)
	})

	t.Run("SearchAndLabels", func(t *testing.T) {
		advisoryRequest(t, "POST", baseURL, adminToken, &api.CreateRepositoryAdvisoryOption{Summary: "x", Description: "x", Labels: []int64{4}}, http.StatusUnprocessableEntity) // organization label of another owner
		created := advisoryJSON(t, "POST", baseURL, adminToken, &api.CreateRepositoryAdvisoryOption{Summary: "SQL injection in search", Description: "details", Labels: []int64{1}}, http.StatusCreated, &api.RepositoryAdvisory{})
		require.Len(t, created.Labels, 1)

		search := func(query string) []string {
			var ids []string
			for _, a := range advisoryJSON(t, "GET", baseURL+"?"+query, adminToken, nil, http.StatusOK, []*api.RepositoryAdvisory{}) {
				ids = append(ids, a.Identifier)
			}
			return ids
		}
		assert.Equal(t, []string{created.Identifier}, search("q=sql+injection"))
		assert.Equal(t, []string{created.Identifier}, search("labels="+created.Labels[0].Name))
		assert.Empty(t, search("labels=unknown"))
		assert.Equal(t, []string{reported.Identifier}, search("severity=critical&ecosystem=go"))
	})

	t.Run("Create", func(t *testing.T) {
		create := &api.CreateRepositoryAdvisoryOption{Summary: "Path traversal", Description: "details", Severity: "low", Credits: []*api.RepositoryAdvisoryCredit{{Login: "user4", Type: "finder"}}}
		advisoryRequest(t, "POST", baseURL, writerToken, create, http.StatusForbidden)
		created := advisoryJSON(t, "POST", baseURL, adminToken, create, http.StatusCreated, &api.RepositoryAdvisory{})
		assert.Equal(t, "draft", created.State)
		require.Len(t, created.Credits, 1)
		assert.Nil(t, created.Submission)

		assert.Equal(t, "3", advisoryRequest(t, "GET", baseURL+"?state=draft", adminToken, nil, http.StatusOK).Header().Get("X-Total-Count"))
		for _, query := range []string{"state=bogus", "close_reason=bogus", "cwe=CWE-7_"} {
			advisoryRequest(t, "GET", baseURL+"?"+query, adminToken, nil, http.StatusUnprocessableEntity)
		}
		create.CVSSVectorString = "CVSS:3.1/AV:N"
		advisoryRequest(t, "POST", baseURL, adminToken, create, http.StatusUnprocessableEntity)
	})

	advisoryRequest(t, "DELETE", pvrURL, adminToken, nil, http.StatusNoContent)
	advisoryRequest(t, "POST", baseURL+"/reports", reporterToken, report, http.StatusForbidden)
}

func TestAPISecurityAdvisorySecurityTeam(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3}) // org3, its team1 has the members user2 and user4
	require.NoError(t, repo_service.UpdateRepositoryUnits(t.Context(), repo, []repo_model.RepoUnit{{RepoID: repo.ID, Type: unit.TypeSecurityAdvisories}}, nil))
	ownerToken := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization)
	memberToken := getUserToken(t, "user4", auth_model.AccessTokenScopeWriteRepository)

	const baseURL = "/api/v1/repos/org3/repo3/security-advisories"
	advisoryURL := baseURL + "/" + advisoryJSON(t, "POST", baseURL, ownerToken, &api.CreateRepositoryAdvisoryOption{Summary: "Draft", Description: "details"}, http.StatusCreated, &api.RepositoryAdvisory{}).Identifier
	advisoryRequest(t, "GET", advisoryURL, memberToken, nil, http.StatusNotFound)

	assert.True(t, advisoryJSON(t, "PATCH", "/api/v1/teams/2", ownerToken, &api.EditTeamOption{IsSecurityTeam: new(true)}, http.StatusOK, &api.Team{}).IsSecurityTeam)
	advisoryRequest(t, "GET", advisoryURL+"/comments", memberToken, nil, http.StatusOK)
	advisoryRequest(t, "PATCH", advisoryURL, memberToken, map[string]any{"summary": "edited by the security team"}, http.StatusOK)
	advisoryRequest(t, "PATCH", advisoryURL, memberToken, map[string]any{"state": "published"}, http.StatusForbidden)
}

func advisoryRequest(t *testing.T, method, url, token string, body any, status int) *httptest.ResponseRecorder {
	t.Helper()
	req := NewRequest(t, method, url)
	if body != nil {
		req = NewRequestWithJSON(t, method, url, body)
	}
	return MakeRequest(t, req.AddTokenAuth(token), status)
}

func advisoryJSON[T any](t *testing.T, method, url, token string, body any, status int, v T) T {
	t.Helper()
	return DecodeJSON(t, advisoryRequest(t, method, url, token, body, status), v)
}

func createTestWebhook(t *testing.T, repoID int64, events *webhook_module.HookEvent) *webhook_model.Webhook {
	t.Helper()
	hook := &webhook_model.Webhook{RepoID: repoID, URL: "http://127.0.0.1:1/", Type: webhook_module.GITEA, ContentType: webhook_model.ContentTypeJSON, IsActive: true, HookEvent: events}
	require.NoError(t, hook.UpdateEvent())
	require.NoError(t, webhook_model.CreateWebhook(t.Context(), hook))
	return hook
}
