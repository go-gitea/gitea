// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bufio"
	"mime"
	"net/http"
	"strings"
	"testing"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Events an admin causes while impersonating must stay traceable to the admin,
// otherwise anything done in an impersonated session is pinned on the victim.
func TestAdminAuditLogImpersonation(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Audit.Enabled, true)()

	session := loginUser(t, "user1")
	session.MakeRequest(t, NewRequest(t, "POST", "/-/admin/users/2/impersonate"), http.StatusOK)

	session.MakeRequest(t, NewRequestWithValues(t, "POST", "/user/settings/applications", map[string]string{
		"name":        "impersonated-token",
		"scope-dummy": "read:user",
	}), http.StatusSeeOther)

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

	exit := byAction[audit_model.UserImpersonationExit]
	require.NotNil(t, exit)
	assert.Equal(t, int64(1), exit.ActorID)
	assert.Zero(t, exit.ImpersonatorID)
}

func TestAdminAuditLogExport(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	adminSession := loginUser(t, "user1")
	userSession := loginUser(t, "user2")
	defer test.MockVariableValue(&setting.Audit.Enabled, true)()

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
