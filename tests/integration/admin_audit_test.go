// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bufio"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"testing"

	audit_model "gitea.dev/models/audit"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminAuditLogAttribution(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Audit.RecordOutput, setting.AuditRecordOutputDatabase)()

	// A leaked token is only traceable if events name the token, not just its owner.
	t.Run("TokenCredential", func(t *testing.T) {
		session := loginUser(t, "user2")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository)

		req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{
			Name: "audit-credential-repo",
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		events, _, err := audit_model.FindEvents(t.Context(), &audit_model.EventSearchOptions{
			Action:  audit_model.RepositoryCreate,
			ActorID: 2,
		})
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.True(t, strings.HasPrefix(events[0].ActorCredential, "access-token:"), "unexpected credential %q", events[0].ActorCredential)
		assert.Equal(t, audit_model.OriginAPI, events[0].Origin)
		assert.NotEmpty(t, events[0].IPAddress)
	})

	// Events an admin causes while impersonating must stay traceable to the admin,
	// otherwise anything done in an impersonated session is pinned on the victim.
	t.Run("Impersonation", func(t *testing.T) {
		session := loginUser(t, "user1")
		session.MakeRequest(t, NewRequest(t, "POST", "/-/admin/users/2/impersonate"), http.StatusOK)

		session.MakeRequest(t, NewRequestWithValues(t, "POST", "/user/settings/applications", map[string]string{
			"name":        "impersonated-token",
			"scope-dummy": "read:user",
		}), http.StatusOK)

		session.MakeRequest(t, NewRequest(t, "GET", "/user/logout"), http.StatusSeeOther)

		events, _, err := audit_model.FindEvents(t.Context(), &audit_model.EventSearchOptions{ActorID: 1})
		require.NoError(t, err)

		byAction := make(map[audit_model.Action]*audit_model.Event, len(events))
		for _, e := range events {
			byAction[e.Action] = e
		}

		start := byAction[audit_model.UserImpersonation]
		require.NotNil(t, start)
		assert.Equal(t, int64(1), start.ActorID)
		assert.Equal(t, int64(2), start.ScopeID)

		token := byAction[audit_model.UserAccessTokenAdd]
		require.NotNil(t, token)
		assert.Equal(t, int64(2), token.ActorID) // the token really belongs to user2
		assert.Equal(t, int64(1), token.ImpersonatorID)
		assert.Equal(t, "user1", token.ImpersonatorName)
		assert.Equal(t, audit_model.OriginUI, token.Origin)
		assert.NotEmpty(t, token.IPAddress)

		exit := byAction[audit_model.UserImpersonationExit]
		require.NotNil(t, exit)
		assert.Equal(t, int64(1), exit.ActorID)
		assert.Zero(t, exit.ImpersonatorID)
	})
}

func TestAdminAuditLogExport(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	adminSession := loginUser(t, "user1")
	userSession := loginUser(t, "user2")
	defer test.MockVariableValue(&setting.Audit.RecordOutput, setting.AuditRecordOutputDatabase)()

	err := audit_model.InsertEvent(t.Context(), &audit_model.Event{
		Action:        audit_model.UserCreate,
		ActorID:       1,
		ActorName:     "audit-export-actor",
		ScopeType:     audit_model.ScopeUser,
		ScopeID:       2,
		ScopeName:     "audit-export-scope",
		Origin:        audit_model.OriginAPI,
		Message:       "Export test event",
		Metadata:      `{"source":"integration-test"}`,
		IPAddress:     "192.0.2.1",
		TimestampUnix: timeutil.TimeStamp(1_700_000_000),
	})
	require.NoError(t, err)

	t.Run("Button", func(t *testing.T) {
		resp := adminSession.MakeRequest(t, NewRequest(t, "GET", "/-/admin/monitor/audit_logs"), http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)
		assert.Equal(t, 1, doc.doc.Find(`a[href="/-/admin/monitor/audit_logs/export"]`).Length())
	})

	t.Run("Filter", func(t *testing.T) {
		resp := adminSession.MakeRequest(t, NewRequest(t, "GET", "/-/admin/monitor/audit_logs?action=user:create&actor=user1&origin=api"), http.StatusOK)
		assert.Contains(t, resp.Body.String(), "Export test event")

		// an actor that did not cause the event filters it out, as does an unknown one
		for _, actor := range []string{"user2", "does-not-exist"} {
			resp = adminSession.MakeRequest(t, NewRequest(t, "GET", "/-/admin/monitor/audit_logs?actor="+actor), http.StatusOK)
			assert.NotContains(t, resp.Body.String(), "Export test event")
		}
	})

	t.Run("FilteredExport", func(t *testing.T) {
		resp := adminSession.MakeRequest(t, NewRequest(t, "GET", "/-/admin/monitor/audit_logs/export?action=repository:create"), http.StatusOK)
		assert.NotContains(t, resp.Body.String(), "Export test event")
	})

	t.Run("AdminOnly", func(t *testing.T) {
		userSession.MakeRequest(t, NewRequest(t, "GET", "/-/admin/monitor/audit_logs/export"), http.StatusForbidden)
	})

	t.Run("JSONL", func(t *testing.T) {
		resp := adminSession.MakeRequest(t, NewRequest(t, "GET", "/-/admin/monitor/audit_logs/export"), http.StatusOK)

		contentType, _, err := mime.ParseMediaType(resp.Header().Get("Content-Type"))
		require.NoError(t, err)
		assert.Equal(t, "application/x-ndjson", contentType)

		disposition, params, err := mime.ParseMediaType(resp.Header().Get("Content-Disposition"))
		require.NoError(t, err)
		assert.Equal(t, "attachment", disposition)
		assert.True(t, strings.HasPrefix(params["filename"], "gitea-audit-log-"))
		assert.True(t, strings.HasSuffix(params["filename"], ".jsonl"))

		found := false
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			var event audit_model.Event
			require.NoError(t, json.Unmarshal(scanner.Bytes(), &event))
			if event.Message == "Export test event" {
				found = true
				assert.Equal(t, audit_model.UserCreate, event.Action)
				assert.Equal(t, "audit-export-actor", event.Actor().Name)
				assert.Equal(t, "audit-export-scope", event.Scope().Name)
				assert.Equal(t, "integration-test", audit_model.DecodeMetadata(event.Metadata)["source"])
				assert.Equal(t, "192.0.2.1", event.IPAddress)
				assert.Equal(t, audit_model.OriginAPI, event.Origin)
			}
		}
		require.NoError(t, scanner.Err())
		assert.True(t, found)
	})
}

func TestAuditLogRepositoryAPIEvents(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Audit.RecordOutput, setting.AuditRecordOutputDatabase)()

	token := getUserToken(t, "user1", auth_model.AccessTokenScopeAll)
	const repoAPI = "/api/v1/repos/user2/repo1"

	assertEvents := func(t *testing.T, scopeType audit_model.ScopeType, scopeID int64, actions ...audit_model.Action) {
		t.Helper()
		for _, action := range actions {
			events, _, err := audit_model.FindEvents(t.Context(), &audit_model.EventSearchOptions{Action: action, ScopeType: scopeType, ScopeID: scopeID})
			require.NoError(t, err)
			assert.Len(t, events, 1, "audit events for %s", action)
		}
	}

	t.Run("Webhooks", func(t *testing.T) {
		hookOption := &api.CreateHookOption{Type: "gitea", Config: api.CreateHookOptionConfig{"url": "http://example.com/hook", "content_type": "json"}}

		resp := MakeRequest(t, NewRequestWithJSON(t, "POST", repoAPI+"/hooks", hookOption).AddTokenAuth(token), http.StatusCreated)
		hookURL := fmt.Sprintf("%s/hooks/%d", repoAPI, DecodeJSON(t, resp, &api.Hook{}).ID)
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", hookURL, &api.EditHookOption{Active: new(false)}).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, "DELETE", hookURL).AddTokenAuth(token), http.StatusNoContent)
		assertEvents(t, audit_model.ScopeRepository, 1, audit_model.RepositoryWebhookAdd, audit_model.RepositoryWebhookUpdate, audit_model.RepositoryWebhookRemove)

		resp = MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/v1/orgs/org3/hooks", hookOption).AddTokenAuth(token), http.StatusCreated)
		MakeRequest(t, NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/orgs/org3/hooks/%d", DecodeJSON(t, resp, &api.Hook{}).ID)).AddTokenAuth(token), http.StatusNoContent)
		assertEvents(t, audit_model.ScopeOrganization, 3, audit_model.OrganizationWebhookAdd, audit_model.OrganizationWebhookRemove)
	})

	t.Run("BranchProtection", func(t *testing.T) {
		MakeRequest(t, NewRequestWithJSON(t, "POST", repoAPI+"/branch_protections", &api.CreateBranchProtectionOption{RuleName: "branch2"}).AddTokenAuth(token), http.StatusCreated)
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", repoAPI+"/branch_protections/branch2", &api.EditBranchProtectionOption{EnablePush: new(true)}).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, "DELETE", repoAPI+"/branch_protections/branch2").AddTokenAuth(token), http.StatusNoContent)
		assertEvents(t, audit_model.ScopeRepository, 1, audit_model.RepositoryBranchProtectionAdd, audit_model.RepositoryBranchProtectionUpdate, audit_model.RepositoryBranchProtectionRemove)
	})

	t.Run("TagProtection", func(t *testing.T) {
		resp := MakeRequest(t, NewRequestWithJSON(t, "POST", repoAPI+"/tag_protections", &api.CreateTagProtectionOption{NamePattern: "v*", WhitelistUsernames: []string{"user2"}}).AddTokenAuth(token), http.StatusCreated)
		tagURL := fmt.Sprintf("%s/tag_protections/%d", repoAPI, DecodeJSON(t, resp, &api.TagProtection{}).ID)
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", tagURL, &api.EditTagProtectionOption{NamePattern: new("release-*")}).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, "DELETE", tagURL).AddTokenAuth(token), http.StatusNoContent)
		assertEvents(t, audit_model.ScopeRepository, 1, audit_model.RepositoryTagProtectionAdd, audit_model.RepositoryTagProtectionUpdate, audit_model.RepositoryTagProtectionRemove)
	})

	t.Run("DeployKeyAndToken", func(t *testing.T) {
		MakeRequest(t, NewRequestWithJSON(t, "POST", repoAPI+"/keys", &api.CreateKeyOption{
			Title: "audit-deploy-key",
			Key:   "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEYv1rUQ6Sx6XQNM5o8URz8xvuXaV9bGXfj+OMkh8T0X",
		}).AddTokenAuth(token), http.StatusCreated)
		MakeRequest(t, NewRequestWithJSON(t, "POST", repoAPI+"/keys/tokens", &api.CreateDeployKeyTokenOption{Title: "audit-deploy-token"}).AddTokenAuth(token), http.StatusCreated)

		events, _, err := audit_model.FindEvents(t.Context(), &audit_model.EventSearchOptions{Action: audit_model.RepositoryDeployKeyAdd, ScopeType: audit_model.ScopeRepository, ScopeID: 1})
		require.NoError(t, err)
		assert.Len(t, events, 2)
	})

	t.Run("RepoSettings", func(t *testing.T) {
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", repoAPI, &api.EditRepoOption{DefaultBranch: new("branch2")}).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", repoAPI, &api.EditRepoOption{Archived: new(true)}).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", repoAPI, &api.EditRepoOption{Archived: new(false)}).AddTokenAuth(token), http.StatusOK)
		assertEvents(t, audit_model.ScopeRepository, 1, audit_model.RepositoryBranchDefault, audit_model.RepositoryArchive, audit_model.RepositoryUnarchive)
	})

	t.Run("Projects", func(t *testing.T) {
		resp := MakeRequest(t, NewRequestWithJSON(t, "POST", repoAPI+"/projects", &api.CreateProjectOption{Title: "audit-project"}).AddTokenAuth(token), http.StatusCreated)
		projectURL := fmt.Sprintf("%s/projects/%d", repoAPI, DecodeJSON(t, resp, &api.Project{}).ID)
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", projectURL, &api.EditProjectOption{Title: new("audit-project-renamed")}).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, "DELETE", projectURL).AddTokenAuth(token), http.StatusNoContent)
		assertEvents(t, audit_model.ScopeRepository, 1, audit_model.ProjectCreate, audit_model.ProjectUpdate, audit_model.ProjectDelete)
	})
}
