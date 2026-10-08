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

	MakeRequest(t, NewRequest(t, "GET", baseURL).AddTokenAuth(adminToken), http.StatusNotFound) // the unit is disabled, also for admins
	require.NoError(t, repo_service.UpdateRepositoryUnits(t.Context(), repo, []repo_model.RepoUnit{{RepoID: repo.ID, Type: unit.TypeSecurityAdvisories}}, nil))

	resp := MakeRequest(t, NewRequest(t, "GET", pvrURL), http.StatusOK)
	assert.False(t, DecodeJSON(t, resp, &api.PrivateVulnerabilityReporting{}).Enabled)
	MakeRequest(t, NewRequestWithJSON(t, "POST", baseURL+"/reports", report).AddTokenAuth(reporterToken), http.StatusForbidden)
	MakeRequest(t, NewRequest(t, "PUT", pvrURL).AddTokenAuth(outsiderToken), http.StatusForbidden)
	MakeRequest(t, NewRequest(t, "PUT", pvrURL).AddTokenAuth(adminToken), http.StatusNoContent)
	// enabling the already enabled unit must keep its config
	MakeRequest(t, NewRequestWithJSON(t, "PATCH", "/api/v1/repos/user2/repo1", &api.EditRepoOption{HasSecurityAdvisories: new(true)}).AddTokenAuth(adminToken), http.StatusOK)
	resp = MakeRequest(t, NewRequest(t, "GET", pvrURL), http.StatusOK)
	assert.True(t, DecodeJSON(t, resp, &api.PrivateVulnerabilityReporting{}).Enabled)

	// webhooks sending everything must not forward undisclosed reports
	everythingHook := createTestWebhook(t, repo.ID, &webhook_module.HookEvent{SendEverything: true})
	reportHook := createTestWebhook(t, repo.ID, &webhook_module.HookEvent{ChooseEvents: true, HookEvents: webhook_module.HookEvents{webhook_module.HookEventRepositoryAdvisoryReported: true}})

	MakeRequest(t, NewRequestWithJSON(t, "POST", baseURL+"/reports", report).AddTokenAuth(blockedToken), http.StatusForbidden)
	resp = MakeRequest(t, NewRequestWithJSON(t, "POST", baseURL+"/reports", report).AddTokenAuth(reporterToken), http.StatusCreated)
	reported := DecodeJSON(t, resp, &api.RepositoryAdvisory{})
	assert.Equal(t, "triage", reported.State)
	assert.Equal(t, "medium", *reported.Severity)
	assert.Equal(t, "user4", reported.Author.UserName)
	assert.False(t, reported.Submission.Accepted)
	advisoryURL := baseURL + "/" + reported.Identifier
	unittest.AssertCount(t, &webhook_model.HookTask{HookID: everythingHook.ID}, 0)
	unittest.AssertExistsAndLoadBean(t, &webhook_model.HookTask{HookID: reportHook.ID, EventType: webhook_module.HookEventRepositoryAdvisoryReported})

	t.Run("DraftVisibility", func(t *testing.T) {
		MakeRequest(t, NewRequest(t, "GET", advisoryURL).AddTokenAuth(writerToken), http.StatusNotFound)
		MakeRequest(t, NewRequest(t, "GET", advisoryURL).AddTokenAuth(adminPublicOnlyToken), http.StatusNotFound)
		MakeRequest(t, NewRequest(t, "GET", advisoryURL), http.StatusNotFound)
		patchAdvisory(t, advisoryURL, outsiderToken, map[string]any{"summary": "x"}, http.StatusNotFound)
		resp := MakeRequest(t, NewRequest(t, "GET", baseURL).AddTokenAuth(adminPublicOnlyToken), http.StatusOK)
		assert.Empty(t, DecodeJSON(t, resp, []*api.RepositoryAdvisory{}))
	})

	t.Run("Edit", func(t *testing.T) {
		// participants can edit the content, but not what only maintainers decide
		patchAdvisory(t, advisoryURL, reporterToken, map[string]any{"state": "draft"}, http.StatusForbidden)
		patchAdvisory(t, advisoryURL, reporterToken, map[string]any{"cve_id": "CVE-2026-1234"}, http.StatusForbidden)
		patchAdvisory(t, advisoryURL, reporterToken, map[string]any{"cwe_ids": []string{"CWE-79"}}, http.StatusOK)
		resp := MakeRequest(t, NewRequest(t, "GET", advisoryURL).AddTokenAuth(reporterToken), http.StatusOK)
		assert.Equal(t, []string{"CWE-79"}, DecodeJSON(t, resp, &api.RepositoryAdvisory{}).CweIDs)

		patchAdvisory(t, advisoryURL, adminToken, map[string]any{"severity": "high"}, http.StatusUnprocessableEntity)
		patchAdvisory(t, advisoryURL, adminToken, map[string]any{"state": "published"}, http.StatusUnprocessableEntity)
		// an invalid collaborator rejects the whole request before anything is saved
		patchAdvisory(t, advisoryURL, adminToken, map[string]any{"summary": "changed", "collaborating_users": []string{"user29"}}, http.StatusForbidden)
		resp = patchAdvisory(t, advisoryURL, adminToken, map[string]any{
			"state":               "draft",
			"description":         "rewritten for publishing",
			"cvss_vector_string":  "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N",
			"collaborating_users": []string{"user5"},
		}, http.StatusOK)
		edited := DecodeJSON(t, resp, &api.RepositoryAdvisory{})
		assert.Equal(t, report.Summary, edited.Summary)
		assert.Equal(t, "draft", edited.State)
		assert.Equal(t, "critical", *edited.Severity)
		assert.NotNil(t, edited.CVSSSeverities.CvssV3, "the 3.1 vector is kept")
		require.Len(t, edited.CollaboratingUsers, 1)
		assert.True(t, edited.Submission.Accepted)
		accepted := unittest.AssertExistsAndLoadBean(t, &advisory_model.Advisory{RepoID: repo.ID, Identifier: reported.Identifier})
		assert.Equal(t, "rewritten for publishing", accepted.Description)
		assert.Equal(t, report.Description, accepted.ReportDescription, "the report is kept as it was accepted")

		MakeRequest(t, NewRequest(t, "GET", advisoryURL).AddTokenAuth(outsiderToken), http.StatusOK)
		patchAdvisory(t, advisoryURL, adminToken, map[string]any{"collaborating_users": []string{}}, http.StatusOK)
		MakeRequest(t, NewRequest(t, "GET", advisoryURL).AddTokenAuth(outsiderToken), http.StatusNotFound)
	})

	t.Run("Comments", func(t *testing.T) {
		commentsURL := advisoryURL + "/comments"
		MakeRequest(t, NewRequest(t, "GET", commentsURL).AddTokenAuth(writerToken), http.StatusNotFound)
		resp := MakeRequest(t, NewRequestWithJSON(t, "POST", commentsURL, &api.RepositoryAdvisoryCommentOption{Body: "steps attached"}).AddTokenAuth(reporterToken), http.StatusCreated)
		reporterComment := DecodeJSON(t, resp, &api.RepositoryAdvisoryComment{})
		resp = MakeRequest(t, NewRequestWithJSON(t, "POST", commentsURL, &api.RepositoryAdvisoryCommentOption{Body: "confirmed"}).AddTokenAuth(adminToken), http.StatusCreated)
		adminComment := DecodeJSON(t, resp, &api.RepositoryAdvisoryComment{})

		adminCommentURL := fmt.Sprintf("%s/%d", commentsURL, adminComment.ID)
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", adminCommentURL, &api.RepositoryAdvisoryCommentOption{Body: "x"}).AddTokenAuth(reporterToken), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, "DELETE", adminCommentURL).AddTokenAuth(reporterToken), http.StatusForbidden)
		resp = MakeRequest(t, NewRequestWithJSON(t, "PATCH", fmt.Sprintf("%s/%d", commentsURL, reporterComment.ID), &api.RepositoryAdvisoryCommentOption{Body: "steps updated"}).AddTokenAuth(reporterToken), http.StatusOK)
		assert.Equal(t, "steps updated", DecodeJSON(t, resp, &api.RepositoryAdvisoryComment{}).Body)
		MakeRequest(t, NewRequest(t, "DELETE", fmt.Sprintf("%s/%d", commentsURL, reporterComment.ID)).AddTokenAuth(adminToken), http.StatusNoContent)

		MakeRequest(t, NewRequestWithJSON(t, "POST", commentsURL, &api.RepositoryAdvisoryCommentOption{Body: "x", IsInternal: true}).AddTokenAuth(reporterToken), http.StatusForbidden)
		resp = MakeRequest(t, NewRequestWithJSON(t, "POST", commentsURL, &api.RepositoryAdvisoryCommentOption{Body: "the reporter seems to sell this", IsInternal: true}).AddTokenAuth(adminToken), http.StatusCreated)
		internalCommentURL := fmt.Sprintf("%s/%d", commentsURL, DecodeJSON(t, resp, &api.RepositoryAdvisoryComment{}).ID)
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", internalCommentURL, &api.RepositoryAdvisoryCommentOption{Body: "x"}).AddTokenAuth(reporterToken), http.StatusNotFound)

		resp = MakeRequest(t, NewRequest(t, "GET", commentsURL).AddTokenAuth(reporterToken), http.StatusOK)
		comments := DecodeJSON(t, resp, []*api.RepositoryAdvisoryComment{})
		require.Len(t, comments, 1)
		assert.Equal(t, "user2", comments[0].User.UserName)
		resp = MakeRequest(t, NewRequest(t, "GET", commentsURL).AddTokenAuth(adminToken), http.StatusOK)
		comments = DecodeJSON(t, resp, []*api.RepositoryAdvisoryComment{})
		require.Len(t, comments, 2)
		assert.True(t, comments[1].IsInternal)
	})

	t.Run("Publish", func(t *testing.T) {
		resp := patchAdvisory(t, advisoryURL, adminToken, map[string]any{"state": "published"}, http.StatusOK)
		assert.Equal(t, "user2", DecodeJSON(t, resp, &api.RepositoryAdvisory{}).Publisher.UserName)

		resp = MakeRequest(t, NewRequest(t, "GET", advisoryURL).AddTokenAuth(outsiderToken), http.StatusOK)
		public := DecodeJSON(t, resp, &api.RepositoryAdvisory{})
		assert.Nil(t, public.Author)
		assert.Nil(t, public.CollaboratingUsers)
		assert.Nil(t, public.Submission)
		MakeRequest(t, NewRequest(t, "GET", advisoryURL).AddTokenAuth(adminPublicOnlyToken), http.StatusOK)
		patchAdvisory(t, advisoryURL, reporterToken, map[string]any{"summary": "x"}, http.StatusForbidden)

		// the published payload is sent without the private parts
		task := unittest.AssertExistsAndLoadBean(t, &webhook_model.HookTask{HookID: everythingHook.ID, EventType: webhook_module.HookEventRepositoryAdvisory})
		assert.Contains(t, task.PayloadContent, `"action": "published"`)
		assert.Contains(t, task.PayloadContent, `"author": null`)
	})

	t.Run("CloseAsDuplicate", func(t *testing.T) {
		resp := MakeRequest(t, NewRequestWithJSON(t, "POST", baseURL, &api.CreateRepositoryAdvisoryOption{Summary: "Original", Description: "details"}).AddTokenAuth(adminToken), http.StatusCreated)
		original := DecodeJSON(t, resp, &api.RepositoryAdvisory{})
		resp = MakeRequest(t, NewRequestWithJSON(t, "POST", baseURL+"/reports", report).AddTokenAuth(outsiderToken), http.StatusCreated)
		duplicateURL := baseURL + "/" + DecodeJSON(t, resp, &api.RepositoryAdvisory{}).Identifier
		patchAdvisory(t, duplicateURL, adminToken, map[string]any{"state": "closed"}, http.StatusUnprocessableEntity)
		patchAdvisory(t, duplicateURL, adminToken, map[string]any{"state": "closed", "close_reason": "duplicate"}, http.StatusUnprocessableEntity)
		resp = patchAdvisory(t, duplicateURL, adminToken, map[string]any{"state": "closed", "close_reason": "duplicate", "duplicate_of": original.Identifier}, http.StatusOK)
		closed := DecodeJSON(t, resp, &api.RepositoryAdvisory{})
		assert.Equal(t, "duplicate", closed.CloseReason)
		assert.Equal(t, original.Identifier, closed.DuplicateOf)

		// the reporter of the duplicate can follow the original, but not edit it
		originalURL := baseURL + "/" + original.Identifier
		MakeRequest(t, NewRequest(t, "GET", originalURL+"/comments").AddTokenAuth(outsiderToken), http.StatusOK)
		patchAdvisory(t, originalURL, outsiderToken, map[string]any{"summary": "x"}, http.StatusForbidden)
		patchAdvisory(t, duplicateURL, adminToken, map[string]any{"state": "draft"}, http.StatusOK)
		MakeRequest(t, NewRequest(t, "GET", originalURL+"/comments").AddTokenAuth(outsiderToken), http.StatusNotFound) // reopening revokes the access
		patchAdvisory(t, duplicateURL, adminToken, map[string]any{"state": "closed", "close_reason": "duplicate", "duplicate_of": original.Identifier}, http.StatusOK)
		resp = MakeRequest(t, NewRequest(t, "GET", baseURL+"?close_reason=duplicate").AddTokenAuth(adminToken), http.StatusOK)
		assert.Len(t, DecodeJSON(t, resp, []*api.RepositoryAdvisory{}), 1)
	})

	t.Run("SearchAndLabels", func(t *testing.T) {
		create := &api.CreateRepositoryAdvisoryOption{Summary: "SQL injection in search", Description: "details", Labels: []int64{1}}
		MakeRequest(t, NewRequestWithJSON(t, "POST", baseURL, &api.CreateRepositoryAdvisoryOption{Summary: "x", Description: "x", Labels: []int64{4}}).AddTokenAuth(adminToken), http.StatusUnprocessableEntity) // organization label of another owner
		resp := MakeRequest(t, NewRequestWithJSON(t, "POST", baseURL, create).AddTokenAuth(adminToken), http.StatusCreated)
		created := DecodeJSON(t, resp, &api.RepositoryAdvisory{})
		require.Len(t, created.Labels, 1)

		search := func(query string) []string {
			resp := MakeRequest(t, NewRequest(t, "GET", baseURL+"?"+query).AddTokenAuth(adminToken), http.StatusOK)
			var ids []string
			for _, a := range DecodeJSON(t, resp, []*api.RepositoryAdvisory{}) {
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
		MakeRequest(t, NewRequestWithJSON(t, "POST", baseURL, create).AddTokenAuth(writerToken), http.StatusForbidden)
		resp := MakeRequest(t, NewRequestWithJSON(t, "POST", baseURL, create).AddTokenAuth(adminToken), http.StatusCreated)
		created := DecodeJSON(t, resp, &api.RepositoryAdvisory{})
		assert.Equal(t, "draft", created.State)
		require.Len(t, created.Credits, 1)
		assert.Nil(t, created.Submission)

		resp = MakeRequest(t, NewRequest(t, "GET", baseURL+"?state=draft").AddTokenAuth(adminToken), http.StatusOK)
		assert.Equal(t, "3", resp.Header().Get("X-Total-Count"))
		for _, query := range []string{"state=bogus", "close_reason=bogus", "cwe=CWE-7_"} {
			MakeRequest(t, NewRequest(t, "GET", baseURL+"?"+query).AddTokenAuth(adminToken), http.StatusUnprocessableEntity)
		}
		create.CVSSVectorString = "CVSS:3.1/AV:N"
		MakeRequest(t, NewRequestWithJSON(t, "POST", baseURL, create).AddTokenAuth(adminToken), http.StatusUnprocessableEntity)
	})

	MakeRequest(t, NewRequest(t, "DELETE", pvrURL).AddTokenAuth(adminToken), http.StatusNoContent)
	MakeRequest(t, NewRequestWithJSON(t, "POST", baseURL+"/reports", report).AddTokenAuth(reporterToken), http.StatusForbidden)
}

func TestAPISecurityAdvisorySecurityTeam(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3}) // org3, its team1 has the members user2 and user4
	require.NoError(t, repo_service.UpdateRepositoryUnits(t.Context(), repo, []repo_model.RepoUnit{{RepoID: repo.ID, Type: unit.TypeSecurityAdvisories}}, nil))
	ownerToken := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization)
	memberToken := getUserToken(t, "user4", auth_model.AccessTokenScopeWriteRepository)

	const baseURL = "/api/v1/repos/org3/repo3/security-advisories"
	resp := MakeRequest(t, NewRequestWithJSON(t, "POST", baseURL, &api.CreateRepositoryAdvisoryOption{Summary: "Draft", Description: "details"}).AddTokenAuth(ownerToken), http.StatusCreated)
	advisoryURL := baseURL + "/" + DecodeJSON(t, resp, &api.RepositoryAdvisory{}).Identifier
	MakeRequest(t, NewRequest(t, "GET", advisoryURL).AddTokenAuth(memberToken), http.StatusNotFound)

	resp = MakeRequest(t, NewRequestWithJSON(t, "PATCH", "/api/v1/teams/2", &api.EditTeamOption{IsSecurityTeam: new(true)}).AddTokenAuth(ownerToken), http.StatusOK)
	assert.True(t, DecodeJSON(t, resp, &api.Team{}).IsSecurityTeam)
	MakeRequest(t, NewRequest(t, "GET", advisoryURL+"/comments").AddTokenAuth(memberToken), http.StatusOK)
	patchAdvisory(t, advisoryURL, memberToken, map[string]any{"summary": "edited by the security team"}, http.StatusOK)
	patchAdvisory(t, advisoryURL, memberToken, map[string]any{"state": "published"}, http.StatusForbidden)
}

func patchAdvisory(t *testing.T, url, token string, body any, status int) *httptest.ResponseRecorder {
	t.Helper()
	return MakeRequest(t, NewRequestWithJSON(t, "PATCH", url, body).AddTokenAuth(token), status)
}

func createTestWebhook(t *testing.T, repoID int64, events *webhook_module.HookEvent) *webhook_model.Webhook {
	t.Helper()
	hook := &webhook_model.Webhook{RepoID: repoID, URL: "http://127.0.0.1:1/", Type: webhook_module.GITEA, ContentType: webhook_model.ContentTypeJSON, IsActive: true, HookEvent: events}
	require.NoError(t, hook.UpdateEvent())
	require.NoError(t, webhook_model.CreateWebhook(t.Context(), hook))
	return hook
}
