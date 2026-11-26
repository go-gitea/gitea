// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

	adminSession := loginUser(t, "user2")
	adminToken := getTokenForLoggedInUser(t, adminSession, auth_model.AccessTokenScopeWriteRepository)
	adminPublicOnlyToken := getTokenForLoggedInUser(t, adminSession, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopePublicOnly)
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
		Vulnerabilities:  []*api.RepositoryAdvisoryVulnerability{{Package: &api.RepositoryAdvisoryPackage{Ecosystem: "go", Name: new("gitea.dev")}, VulnerableVersionRange: new("< 1.2.3")}},
	}

	advisoryRequest(t, "GET", baseURL, adminToken, nil, http.StatusNotFound)
	require.NoError(t, repo_service.UpdateRepositoryUnits(t.Context(), repo, []repo_model.RepoUnit{{RepoID: repo.ID, Type: unit.TypeSecurityAdvisories}}, nil))

	assert.False(t, advisoryJSON(t, "GET", pvrURL, "", nil, http.StatusOK, &api.PrivateVulnerabilityReporting{}).Enabled)
	advisoryRequest(t, "POST", baseURL+"/reports", reporterToken, report, http.StatusForbidden)
	advisoryRequest(t, "PUT", pvrURL, outsiderToken, nil, http.StatusForbidden)
	advisoryRequest(t, "PUT", pvrURL, adminToken, nil, http.StatusNoContent)
	advisoryRequest(t, "PATCH", "/api/v1/repos/user2/repo1", adminToken, &api.EditRepoOption{HasSecurityAdvisories: new(true)}, http.StatusOK)
	assert.True(t, advisoryJSON(t, "GET", pvrURL, "", nil, http.StatusOK, &api.PrivateVulnerabilityReporting{}).Enabled)

	everythingHook := createTestWebhook(t, repo.ID, &webhook_module.HookEvent{SendEverything: true})
	reportHook := createTestWebhook(t, repo.ID, &webhook_module.HookEvent{ChooseEvents: true, HookEvents: webhook_module.HookEvents{webhook_module.HookEventRepositoryAdvisoryReported: true}})

	advisoryRequest(t, "POST", baseURL+"/reports", blockedToken, report, http.StatusForbidden)
	report.StartPrivateFork = true
	rejected := advisoryJSON(t, "POST", baseURL+"/reports", reporterToken, report, http.StatusUnprocessableEntity, &api.APIError{})
	assert.Equal(t, "temporary private forks are not supported", rejected.Message)
	report.StartPrivateFork = false
	response := advisoryRequest(t, "POST", baseURL+"/reports", reporterToken, report, http.StatusCreated)
	assert.Contains(t, response.Body.String(), `"gtsa_id":`)
	assert.NotContains(t, response.Body.String(), `"identifier":`)
	reported := DecodeJSON(t, response, &api.RepositoryAdvisory{})
	assert.NotEmpty(t, reported.Identifier)
	assert.Equal(t, "triage", reported.State)
	assert.Equal(t, "medium", *reported.Severity)
	assert.Equal(t, "user4", reported.Author.UserName)
	assert.False(t, reported.Submission.Accepted)
	advisoryURL := baseURL + "/" + reported.Identifier
	unittest.AssertCount(t, &webhook_model.HookTask{HookID: everythingHook.ID}, 0)
	reportTask := unittest.AssertExistsAndLoadBean(t, &webhook_model.HookTask{HookID: reportHook.ID, EventType: webhook_module.HookEventRepositoryAdvisoryReported})

	t.Run("PrivateReportWebhook", func(t *testing.T) {
		const hooksURL = "/api/v1/repos/user2/repo1/hooks"
		hookURL := fmt.Sprintf("%s/%d", hooksURL, reportHook.ID)
		deliveriesURL := hookURL + "/deliveries"
		event := string(webhook_module.HookEventRepositoryAdvisoryReported)
		assert.Contains(t, reportTask.PayloadContent, `"gtsa_id": "`+reported.Identifier+`"`)
		advisoryRequest(t, "POST", hooksURL, adminPublicOnlyToken, &api.CreateHookOption{
			Type: webhook_module.GITEA, Config: api.CreateHookOptionConfig{"content_type": "json", "url": reportHook.URL}, Events: []string{event},
		}, http.StatusForbidden)
		advisoryRequest(t, "PATCH", fmt.Sprintf("%s/%d", hooksURL, everythingHook.ID), adminPublicOnlyToken, &api.EditHookOption{Events: []string{event}}, http.StatusForbidden)
		advisoryRequest(t, "PATCH", hookURL, adminPublicOnlyToken, &api.EditHookOption{Name: new("New name")}, http.StatusForbidden)
		deliveries := advisoryRequest(t, "GET", deliveriesURL, adminPublicOnlyToken, nil, http.StatusOK)
		assert.Equal(t, "0", deliveries.Header().Get("X-Total-Count"))
		assert.Empty(t, DecodeJSON(t, deliveries, api.HookDeliveryList{}))
		advisoryRequest(t, "GET", deliveriesURL+"/"+reportTask.UUID, adminPublicOnlyToken, nil, http.StatusNotFound)
		assert.Equal(t, event, advisoryJSON(t, "GET", deliveriesURL+"/"+reportTask.UUID, adminToken, nil, http.StatusOK, &api.HookDelivery{}).EventType)
	})

	t.Run("DraftVisibility", func(t *testing.T) {
		advisoryRequest(t, "GET", advisoryURL, writerToken, nil, http.StatusNotFound)
		advisoryRequest(t, "GET", advisoryURL, adminPublicOnlyToken, nil, http.StatusNotFound)
		advisoryRequest(t, "GET", advisoryURL, "", nil, http.StatusNotFound)
		advisoryRequest(t, "PATCH", advisoryURL, outsiderToken, map[string]any{"summary": "x"}, http.StatusNotFound)
		assert.Empty(t, advisoryJSON(t, "GET", baseURL, adminPublicOnlyToken, nil, http.StatusOK, []*api.RepositoryAdvisory{}))
	})

	t.Run("Edit", func(t *testing.T) {
		advisoryRequest(t, "PATCH", advisoryURL, reporterToken, map[string]any{"state": "draft"}, http.StatusForbidden)
		advisoryRequest(t, "PATCH", advisoryURL, reporterToken, map[string]any{"cve_id": "CVE-2026-1234"}, http.StatusForbidden)
		reporterEdit := advisoryJSON(t, "PATCH", advisoryURL, reporterToken, map[string]any{"cwe_ids": []string{"CWE-79"}}, http.StatusOK, &api.RepositoryAdvisory{})
		assert.Equal(t, []string{"CWE-79"}, reporterEdit.CweIDs)

		advisoryRequest(t, "PATCH", advisoryURL, adminToken, map[string]any{"state": "published"}, http.StatusUnprocessableEntity)
		advisoryRequest(t, "PATCH", advisoryURL, adminToken, map[string]any{"summary": "changed", "collaborating_users": []string{"user29"}}, http.StatusForbidden)
		edited := advisoryJSON(t, "PATCH", advisoryURL, adminToken, map[string]any{
			"state":               "draft",
			"description":         "rewritten for publishing",
			"cvss_vector_string":  "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N",
			"collaborating_users": []string{"user5"},
		}, http.StatusOK, &api.RepositoryAdvisory{})
		assert.Equal(t, report.Summary, edited.Summary)
		assert.Equal(t, reporterEdit.CweIDs, edited.CweIDs)
		assert.Equal(t, "draft", edited.State)
		assert.Equal(t, "critical", *edited.Severity)
		assert.Nil(t, edited.CVSS)
		assert.Nil(t, edited.CVSSSeverities.CvssV3)
		assert.NotNil(t, edited.CVSSSeverities.CvssV4)
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
		comment := advisoryJSON(t, "GET", adminCommentURL, reporterToken, nil, http.StatusOK, &api.RepositoryAdvisoryComment{})
		assert.Equal(t, "confirmed", comment.Body)
		assert.Equal(t, reported.URL, comment.AdvisoryURL)
		assert.Equal(t, fmt.Sprintf("%s/comments/%d", reported.URL, comment.ID), comment.URL)

		advisoryRequest(t, "PATCH", adminCommentURL, reporterToken, &api.RepositoryAdvisoryCommentOption{Body: "x"}, http.StatusForbidden)
		advisoryRequest(t, "DELETE", adminCommentURL, reporterToken, nil, http.StatusForbidden)
		assert.Equal(t, "steps updated", advisoryJSON(t, "PATCH", reporterCommentURL, reporterToken, &api.RepositoryAdvisoryCommentOption{Body: "steps updated"}, http.StatusOK, &api.RepositoryAdvisoryComment{}).Body)
		advisoryRequest(t, "DELETE", reporterCommentURL, adminToken, nil, http.StatusNoContent)

		advisoryRequest(t, "POST", commentsURL, reporterToken, &api.RepositoryAdvisoryCommentOption{Body: "x", IsConfidential: true}, http.StatusForbidden)
		confidentialCommentURL := postComment(adminToken, &api.RepositoryAdvisoryCommentOption{Body: "the reporter seems to sell this", IsConfidential: true})
		advisoryRequest(t, "GET", confidentialCommentURL, reporterToken, nil, http.StatusNotFound)

		comments := advisoryJSON(t, "GET", commentsURL+"?since="+comment.UpdatedAt.Add(-time.Second).UTC().Format(time.RFC3339), reporterToken, nil, http.StatusOK, []*api.RepositoryAdvisoryComment{})
		require.Len(t, comments, 1)
		assert.Equal(t, "user2", comments[0].User.UserName)
		assert.Empty(t, advisoryJSON(t, "GET", commentsURL+"?since="+comment.UpdatedAt.UTC().Format(time.RFC3339), reporterToken, nil, http.StatusOK, []*api.RepositoryAdvisoryComment{}))
		comments = advisoryJSON(t, "GET", commentsURL, adminToken, nil, http.StatusOK, []*api.RepositoryAdvisoryComment{})
		require.Len(t, comments, 2)
		assert.True(t, comments[1].IsConfidential)
	})

	t.Run("Publish", func(t *testing.T) {
		assert.Equal(t, "user2", advisoryJSON(t, "PATCH", advisoryURL, adminToken, map[string]any{"state": "published"}, http.StatusOK, &api.RepositoryAdvisory{}).Publisher.UserName)
		public := advisoryJSON(t, "GET", advisoryURL, outsiderToken, nil, http.StatusOK, &api.RepositoryAdvisory{})
		assert.Nil(t, public.Author)
		assert.Nil(t, public.CollaboratingUsers)
		assert.Nil(t, public.Submission)
		advisoryRequest(t, "GET", advisoryURL, adminPublicOnlyToken, nil, http.StatusOK)
		advisoryRequest(t, "PATCH", advisoryURL, reporterToken, map[string]any{"summary": "x"}, http.StatusForbidden)

		task := unittest.AssertExistsAndLoadBean(t, &webhook_model.HookTask{HookID: everythingHook.ID, EventType: webhook_module.HookEventRepositoryAdvisory})
		assert.Contains(t, task.PayloadContent, `"action": "published"`)
		assert.Contains(t, task.PayloadContent, `"author": null`)
	})

	t.Run("PrivateReporterCredit", func(t *testing.T) {
		privateToken := getUserToken(t, "user31", auth_model.AccessTokenScopeWriteRepository)
		privateReport := advisoryJSON(t, "POST", baseURL+"/reports", privateToken, report, http.StatusCreated, &api.RepositoryAdvisory{})
		privateURL := baseURL + "/" + privateReport.Identifier
		advisoryRequest(t, "PATCH", privateURL, adminToken, map[string]any{"state": "draft"}, http.StatusOK)
		advisoryRequest(t, "PATCH", privateURL, adminToken, map[string]any{"state": "published"}, http.StatusOK)

		assert.Empty(t, advisoryJSON(t, "GET", privateURL, "", nil, http.StatusOK, &api.RepositoryAdvisory{}).Credits)
		assert.Len(t, advisoryJSON(t, "GET", privateURL, privateToken, nil, http.StatusOK, &api.RepositoryAdvisory{}).Credits, 1)
		page := MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/security/advisories/"+privateReport.Identifier), http.StatusOK)
		assert.NotContains(t, page.Body.String(), "user31")
	})

	t.Run("CloseAsDuplicate", func(t *testing.T) {
		original := advisoryJSON(t, "POST", baseURL, adminToken, &api.CreateRepositoryAdvisoryOption{Summary: "Original", Description: "details"}, http.StatusCreated, &api.RepositoryAdvisory{})
		originalURL := baseURL + "/" + original.Identifier
		duplicateURL := baseURL + "/" + advisoryJSON(t, "POST", baseURL+"/reports", outsiderToken, report, http.StatusCreated, &api.RepositoryAdvisory{}).Identifier
		closeAsDuplicate := map[string]any{"state": "closed", "close_reason": "duplicate", "duplicate_of": original.Identifier}
		advisoryRequest(t, "PATCH", duplicateURL, adminToken, map[string]any{"state": "closed", "close_reason": "duplicate"}, http.StatusUnprocessableEntity)
		closed := advisoryJSON(t, "PATCH", duplicateURL, adminToken, closeAsDuplicate, http.StatusOK, &api.RepositoryAdvisory{})
		assert.Equal(t, "duplicate", closed.CloseReason)
		assert.Equal(t, original.Identifier, closed.DuplicateOf)

		assert.Equal(t, original.Identifier, advisoryJSON(t, "GET", duplicateURL, outsiderToken, nil, http.StatusOK, &api.RepositoryAdvisory{}).DuplicateOf)
		advisoryRequest(t, "GET", originalURL+"/comments", outsiderToken, nil, http.StatusNotFound)
		assert.Len(t, advisoryJSON(t, "GET", baseURL+"?close_reason=duplicate", adminToken, nil, http.StatusOK, []*api.RepositoryAdvisory{}), 1)
	})

	t.Run("SearchAndLabels", func(t *testing.T) {
		advisoryRequest(t, "POST", baseURL, adminToken, &api.CreateRepositoryAdvisoryOption{Summary: "x", Description: "x", Labels: []int64{4}}, http.StatusUnprocessableEntity)
		created := advisoryJSON(t, "POST", baseURL, adminToken, &api.CreateRepositoryAdvisoryOption{Summary: "SQL injection in search", Description: "details", Labels: []int64{1}}, http.StatusCreated, &api.RepositoryAdvisory{})
		require.Len(t, created.Labels, 1)

		search := func(query string) []string {
			var ids []string
			for _, advisory := range advisoryJSON(t, "GET", baseURL+"?"+query, adminToken, nil, http.StatusOK, []*api.RepositoryAdvisory{}) {
				ids = append(ids, advisory.Identifier)
			}
			return ids
		}
		assert.Equal(t, []string{created.Identifier}, search("q=sql+injection"))
		assert.Equal(t, []string{created.Identifier}, search("labels="+created.Labels[0].Name))
		assert.Empty(t, search("labels=unknown"))
		assert.Equal(t, []string{reported.Identifier}, search("severity=critical&ecosystem=go"))
	})

	t.Run("Create", func(t *testing.T) {
		create := &api.CreateRepositoryAdvisoryOption{
			Summary: "Path traversal", Description: "details", Severity: "low",
			Credits:         []*api.RepositoryAdvisoryCredit{{Login: "user4", Type: "finder"}},
			Vulnerabilities: []*api.RepositoryAdvisoryVulnerability{{Package: &api.RepositoryAdvisoryPackage{Ecosystem: "go"}}},
		}
		advisoryRequest(t, "POST", baseURL, writerToken, create, http.StatusForbidden)
		create.StartPrivateFork = true
		rejected := advisoryJSON(t, "POST", baseURL, adminToken, create, http.StatusUnprocessableEntity, &api.APIError{})
		assert.Equal(t, "temporary private forks are not supported", rejected.Message)
		create.StartPrivateFork = false
		response := advisoryRequest(t, "POST", baseURL, adminToken, create, http.StatusCreated)
		for _, field := range []string{"cve_id", "name", "vulnerable_version_range", "patched_versions"} {
			assert.Contains(t, response.Body.String(), fmt.Sprintf("%q:null", field))
		}
		created := DecodeJSON(t, response, &api.RepositoryAdvisory{})
		assert.Equal(t, "draft", created.State)
		require.Len(t, created.Credits, 1)
		assert.Nil(t, created.Submission)

		assert.Equal(t, "3", advisoryRequest(t, "GET", baseURL+"?state=draft", adminToken, nil, http.StatusOK).Header().Get("X-Total-Count"))
		for _, query := range []string{"state=bogus", "close_reason=bogus", "cwe=CWE-7_"} {
			advisoryRequest(t, "GET", baseURL+"?"+query, adminToken, nil, http.StatusUnprocessableEntity)
		}
	})

	advisoryRequest(t, "DELETE", pvrURL, adminToken, nil, http.StatusNoContent)
	advisoryRequest(t, "POST", baseURL+"/reports", reporterToken, report, http.StatusForbidden)
}

func TestAPISecurityAdvisorySecurityTeam(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	require.NoError(t, repo_service.UpdateRepositoryUnits(t.Context(), repo, []repo_model.RepoUnit{{RepoID: repo.ID, Type: unit.TypeSecurityAdvisories}}, nil))
	ownerToken := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization)
	memberToken := getUserToken(t, "user4", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeReadOrganization)

	const baseURL = "/api/v1/repos/org3/repo3/security-advisories"
	draft := advisoryJSON(t, "POST", baseURL, ownerToken, &api.CreateRepositoryAdvisoryOption{
		Summary: "Draft", Description: "details", Labels: []int64{4, 10},
	}, http.StatusCreated, &api.RepositoryAdvisory{})
	advisoryURL := baseURL + "/" + draft.Identifier
	advisoryRequest(t, "GET", advisoryURL, memberToken, nil, http.StatusNotFound)

	assert.True(t, advisoryJSON(t, "PATCH", "/api/v1/teams/2", ownerToken, &api.EditTeamOption{IsSecurityTeam: new(true)}, http.StatusOK, &api.Team{}).IsSecurityTeam)
	advisoryRequest(t, "GET", advisoryURL+"/comments", memberToken, nil, http.StatusOK)
	advisoryRequest(t, "PATCH", advisoryURL, memberToken, map[string]any{"summary": "edited by the security team"}, http.StatusOK)
	advisoryRequest(t, "PATCH", advisoryURL, memberToken, map[string]any{"state": "published"}, http.StatusForbidden)

	otherRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 32})
	require.NoError(t, repo_service.UpdateRepositoryUnits(t.Context(), otherRepo, []repo_model.RepoUnit{{RepoID: otherRepo.ID, Type: unit.TypeSecurityAdvisories}}, nil))
	const otherRepoURL = "/api/v1/repos/org3/repo21"
	advisoryRequest(t, "PUT", otherRepoURL+"/private-vulnerability-reporting", ownerToken, nil, http.StatusNoContent)
	report := advisoryJSON(t, "POST", otherRepoURL+"/security-advisories/reports", memberToken, &api.CreatePrivateVulnerabilityReportOption{
		Summary: "Report", Description: "details",
	}, http.StatusCreated, &api.RepositoryAdvisory{})
	reportURL := otherRepoURL + "/security-advisories/" + report.Identifier
	assigned := advisoryJSON(t, "PATCH", reportURL, ownerToken, &api.EditRepositoryAdvisoryOption{
		Labels: []int64{4}, CollaboratingTeams: []string{"team1", "test_team"},
	}, http.StatusOK, &api.RepositoryAdvisory{})
	require.Len(t, assigned.CollaboratingTeams, 2)
	visible := advisoryJSON(t, "GET", reportURL, memberToken, nil, http.StatusOK, &api.RepositoryAdvisory{}).CollaboratingTeams
	require.Len(t, visible, 1)
	assert.Equal(t, "team1", visible[0].Name)
	closed := advisoryJSON(t, "PATCH", reportURL, ownerToken, map[string]any{"state": "closed"}, http.StatusOK, &api.RepositoryAdvisory{})
	assert.Equal(t, "closed", closed.State)
	assert.Empty(t, closed.CloseReason)

	const orgURL = "/api/v1/orgs/org3/security-advisories"
	advisoryRequest(t, "GET", orgURL, memberToken, nil, http.StatusForbidden)
	response := advisoryRequest(t, "GET", orgURL+"?sort=created&direction=asc&limit=1&page=2", ownerToken, nil, http.StatusOK)
	assert.Equal(t, "2", response.Header().Get("X-Total-Count"))
	page := DecodeJSON(t, response, []*api.RepositoryAdvisory{})
	require.Len(t, page, 1)
	assert.Equal(t, closed.URL, page[0].URL)
	drafts := advisoryJSON(t, "GET", orgURL+"?state=draft", ownerToken, nil, http.StatusOK, []*api.RepositoryAdvisory{})
	require.Len(t, drafts, 1)
	assert.Equal(t, draft.URL, drafts[0].URL)
}

func advisoryRequest(t *testing.T, method, url, token string, body any, status int) *httptest.ResponseRecorder {
	t.Helper()
	if body != nil {
		return MakeRequest(t, NewRequestWithJSON(t, method, url, body).AddTokenAuth(token), status)
	}
	return MakeRequest(t, NewRequest(t, method, url).AddTokenAuth(token), status)
}

func advisoryJSON[T any](t *testing.T, method, url, token string, body any, status int, output T) T {
	t.Helper()
	return DecodeJSON(t, advisoryRequest(t, method, url, token, body, status), output)
}

func createTestWebhook(t *testing.T, repoID int64, events *webhook_module.HookEvent) *webhook_model.Webhook {
	t.Helper()
	hook := &webhook_model.Webhook{RepoID: repoID, URL: "http://127.0.0.1:1/", Type: webhook_module.GITEA, ContentType: webhook_model.ContentTypeJSON, IsActive: true, HookEvent: events}
	require.NoError(t, hook.UpdateEvent())
	require.NoError(t, webhook_model.CreateWebhook(t.Context(), hook))
	return hook
}
