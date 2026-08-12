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
