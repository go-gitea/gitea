// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bufio"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"testing"

	asymkey_model "gitea.dev/models/asymkey"
	audit_model "gitea.dev/models/audit"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"
	"gitea.dev/services/auth/source/oauth2"
	"gitea.dev/tests"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
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

func TestAuditLogAdministrationEvents(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Audit.RecordOutput, setting.AuditRecordOutputDatabase)()

	token := getUserToken(t, "user1", auth_model.AccessTokenScopeAll)
	const repoAPI = "/api/v1/repos/user2/repo1"

	assertEvent := func(t *testing.T, action audit_model.Action, scopeType audit_model.ScopeType, scopeID int64) {
		t.Helper()
		unittest.AssertCount(t, &audit_model.Event{Action: action, ScopeType: scopeType, ScopeID: scopeID}, 1)
	}

	t.Run("ActionsRunners", func(t *testing.T) {
		MakeRequest(t, NewRequest(t, "POST", repoAPI+"/actions/runners/registration-token").AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", repoAPI+"/actions/runners/34348", &api.EditActionRunnerOption{Disabled: new(true)}).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, "DELETE", repoAPI+"/actions/runners/34348").AddTokenAuth(token), http.StatusNoContent)
		for _, action := range []audit_model.Action{audit_model.RepositoryRunnerToken, audit_model.RepositoryRunnerUpdate, audit_model.RepositoryRunnerRemove} {
			assertEvent(t, action, audit_model.ScopeRepository, 1)
		}
	})

	t.Run("ActionsVariables", func(t *testing.T) {
		MakeRequest(t, NewRequestWithJSON(t, "POST", repoAPI+"/actions/variables/AUDIT_VAR", &api.CreateVariableOption{Value: "a"}).AddTokenAuth(token), http.StatusCreated)
		MakeRequest(t, NewRequestWithJSON(t, "PUT", repoAPI+"/actions/variables/AUDIT_VAR", &api.UpdateVariableOption{Value: "b"}).AddTokenAuth(token), http.StatusNoContent)
		MakeRequest(t, NewRequest(t, "DELETE", repoAPI+"/actions/variables/AUDIT_VAR").AddTokenAuth(token), http.StatusNoContent)
		for _, action := range []audit_model.Action{audit_model.RepositoryVariableAdd, audit_model.RepositoryVariableUpdate, audit_model.RepositoryVariableRemove} {
			assertEvent(t, action, audit_model.ScopeRepository, 1)
		}
	})

	t.Run("SystemConfig", func(t *testing.T) {
		session := loginUser(t, "user1")
		for range 2 {
			session.MakeRequest(t, NewRequestWithValues(t, "POST", "/-/admin/config", map[string]string{
				"key":   "instance.web_banner",
				"value": `{"DisplayEnabled":true,"ContentMessage":"audit"}`,
			}), http.StatusOK)
		}
		assertEvent(t, audit_model.SystemConfigUpdate, audit_model.ScopeSystem, 0)
	})

	t.Run("PullReviewDelete", func(t *testing.T) {
		MakeRequest(t, NewRequest(t, "DELETE", repoAPI+"/pulls/2/reviews/1").AddTokenAuth(token), http.StatusNoContent)
		assertEvent(t, audit_model.PullRequestReviewDelete, audit_model.ScopeRepository, 1)
	})

	t.Run("OAuth2ApplicationSecret", func(t *testing.T) {
		option := &api.CreateOAuth2ApplicationOptions{Name: "audit-app", RedirectURIs: []string{"https://example.com/callback"}}
		resp := MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/v1/user/applications/oauth2", option).AddTokenAuth(token), http.StatusCreated)
		app := DecodeJSON(t, resp, &api.OAuth2Application{})
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/user/applications/oauth2/%d", app.ID), option).AddTokenAuth(token), http.StatusOK)
		assertEvent(t, audit_model.UserOAuth2ApplicationSecret, audit_model.ScopeUser, 1)
	})
}

func TestAuditLogUserEvents(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Audit.RecordOutput, setting.AuditRecordOutputDatabase)()

	findEvent := func(t *testing.T, action audit_model.Action, scopeID int64) *audit_model.Event {
		t.Helper()
		events, _, err := audit_model.FindEvents(t.Context(), &audit_model.EventSearchOptions{Action: action, ScopeType: audit_model.ScopeUser, ScopeID: scopeID})
		require.NoError(t, err)
		require.Len(t, events, 1, "audit events for %s", action)
		return events[0]
	}

	t.Run("EmailPrimaryAndActivate", func(t *testing.T) {
		email, err := user_model.InsertEmailAddress(t.Context(), &user_model.EmailAddress{UID: 2, Email: "audit-primary@example.com", IsActivated: true})
		require.NoError(t, err)

		loginUser(t, "user2").MakeRequest(t, NewRequestWithValues(t, "POST", "/user/settings/account/email", map[string]string{
			"_method": "PRIMARY",
			"id":      strconv.FormatInt(email.ID, 10),
		}), http.StatusSeeOther)

		assert.Equal(t, email.Email, audit_model.DecodeMetadata(findEvent(t, audit_model.UserEmailPrimaryChange, 2).Metadata)["email"])

		code := user_model.GenerateUserTimeLimitCode(&user_model.TimeLimitCodeOptions{Purpose: user_model.TimeLimitCodeActivateEmail, NewEmail: "user2-2@example.com"}, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}))
		MakeRequest(t, NewRequest(t, "GET", "/user/activate_email?code="+code+"&email=user2-2%40example.com"), http.StatusSeeOther)
		assert.Equal(t, "user2-2@example.com", audit_model.DecodeMetadata(findEvent(t, audit_model.UserEmailActivate, 2).Metadata)["email"])
	})

	t.Run("APIGPGKey", func(t *testing.T) {
		token := getTokenForLoggedInUser(t, loginUser(t, "user2"), auth_model.AccessTokenScopeWriteUser)
		testCreateValidGPGKey(t, MakeRequest, token, http.StatusCreated)
		findEvent(t, audit_model.UserKeyGPGAdd, 2)

		key := unittest.AssertExistsAndLoadBean(t, &asymkey_model.GPGKey{OwnerID: 2, KeyID: "38EA3BCED732982C"})
		MakeRequest(t, NewRequest(t, "DELETE", "/api/v1/user/gpg_keys/"+strconv.FormatInt(key.ID, 10)).AddTokenAuth(token), http.StatusNoContent)
		findEvent(t, audit_model.UserKeyGPGRemove, 2)
	})

	t.Run("TwoFactorFailOnSignInAndPasswordReset", func(t *testing.T) {
		loginUser(t, "user24").MakeRequest(t, NewRequestWithValues(t, "POST", "/user/two_factor/scratch", map[string]string{
			"token": "wrong-token",
		}), http.StatusOK)
		findEvent(t, audit_model.UserAuthenticationFailTwoFactor, 24)

		code := user_model.GenerateUserTimeLimitCode(&user_model.TimeLimitCodeOptions{Purpose: user_model.TimeLimitCodeResetPassword}, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 24}))
		for _, scratchCode := range []string{"true", "false"} {
			MakeRequest(t, NewRequestWithValues(t, "POST", "/user/recover_account", map[string]string{
				"code":         code,
				"scratch_code": scratchCode,
				"token":        "wrong-token",
			}), http.StatusOK)
		}
		unittest.AssertCount(t, &audit_model.Event{Action: audit_model.UserAuthenticationFailTwoFactor, ScopeID: 24, ActorID: 24}, 3)
	})

	t.Run("PasswordResetActor", func(t *testing.T) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
		code := user_model.GenerateUserTimeLimitCode(&user_model.TimeLimitCodeOptions{Purpose: user_model.TimeLimitCodeResetPassword}, user)
		MakeRequest(t, NewRequestWithValues(t, "POST", "/user/recover_account", map[string]string{
			"code":     code,
			"password": "examplePassword!1",
		}), http.StatusSeeOther)
		assert.Equal(t, user.ID, findEvent(t, audit_model.UserPassword, user.ID).ActorID)
	})

	t.Run("AdminResetTwoFactor", func(t *testing.T) {
		session := loginUser(t, "user1")
		for range 2 {
			session.MakeRequest(t, NewRequestWithValues(t, "POST", "/-/admin/users/24/edit", map[string]string{
				"login_name": "user24",
				"login_type": "0-0",
				"email":      "user24@example.com",
				"active":     "on",
				"reset_2fa":  "on",
			}), http.StatusSeeOther)
		}
		assert.Equal(t, int64(1), findEvent(t, audit_model.UserTwoFactorDisable, 24).ActorID)
	})

	t.Run("OAuth2AutoRegisterWithoutAccountLinking", func(t *testing.T) {
		defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
		defer test.MockVariableValue(&setting.OAuth2Client.AccountLinking, setting.OAuth2AccountLinkingDisabled)()
		defer test.MockVariableValue(&gothic.CompleteUserAuth, func(http.ResponseWriter, *http.Request) (goth.User, error) {
			return goth.User{Provider: "gitea", UserID: "audit-oauth2", NickName: "audit-oauth2", Email: "audit-oauth2@example.com"}, nil
		})()
		addOAuth2Source(t, "audit-oauth2", oauth2.Source{})

		for range 2 {
			emptyTestSession(t).MakeRequest(t, NewRequest(t, "GET", "/user/oauth2/audit-oauth2/callback?code=XYZ&state=XYZ"), http.StatusSeeOther)
		}
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{LoginName: "audit-oauth2"})
		assert.Equal(t, "audit-oauth2", audit_model.DecodeMetadata(findEvent(t, audit_model.UserExternalLoginAdd, user.ID).Metadata)["external_id"])
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
			assert.Equal(t, 1, unittest.GetCount(t, &audit_model.Event{Action: action, ScopeType: scopeType, ScopeID: scopeID}), "audit events for %s", action)
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

		unittest.AssertCount(t, &audit_model.Event{Action: audit_model.RepositoryDeployKeyAdd, ScopeType: audit_model.ScopeRepository, ScopeID: 1}, 2)
	})

	t.Run("RepoSettings", func(t *testing.T) {
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", repoAPI, &api.EditRepoOption{DefaultBranch: new("branch2"), Archived: new(false)}).AddTokenAuth(token), http.StatusOK)
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
