// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bufio"
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
		loginUser(t, "user1").MakeRequest(t, NewRequestWithValues(t, "POST", "/-/admin/users/24/edit", map[string]string{
			"login_name": "user24",
			"login_type": "0-0",
			"email":      "user24@example.com",
			"active":     "on",
			"reset_2fa":  "on",
		}), http.StatusSeeOther)
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
