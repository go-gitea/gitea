// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"net/http"
	"testing"

	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestCodespaceActionHelpers(t *testing.T) {
	t.Run("return path", func(t *testing.T) {
		tests := []struct {
			input string
			want  string
		}{
			{"", "/-/codespaces/uuid"},
			{"/-/codespaces?state=deleted", "/-/codespaces?state=deleted"},
			{"/-/codespaces/uuid?tab=logs", "/-/codespaces/uuid?tab=logs"},
			{"/-/codespaces/other", "/-/codespaces/uuid"},
			{"https://example.com/", "/-/codespaces/uuid"},
			{"//example.com/", "/-/codespaces/uuid"},
			{"relative", "/-/codespaces/uuid"},
		}
		for _, test := range tests {
			assert.Equal(t, test.want, codespaceActionReturnPath("uuid", test.input, codespaceDetailPath("uuid")))
		}
		assert.Equal(t, "/-/codespaces?owner=org&page=2", codespaceListPath("org", 2))
	})

	t.Run("auto-stop duration form", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "POST /-/codespaces/test/auto-stop")
		tests := []struct {
			value string
			unit  string
			want  int64
			ok    bool
		}{
			{"30", "seconds", 30, true},
			{"5", "minutes", 300, true},
			{"2", "hours", 7200, true},
			{"7", "days", 604800, true},
			{"0", "minutes", 0, false},
			{"invalid", "minutes", 0, false},
			{"1", "weeks", 0, false},
			{"9223372036854775807", "days", 0, false},
		}
		for _, test := range tests {
			ctx.Req.Form.Set("timeout_value", test.value)
			ctx.Req.Form.Set("timeout_unit", test.unit)
			value, ok := parseAutoStopTimeoutForm(ctx)
			assert.Equal(t, test.ok, ok)
			assert.Equal(t, test.want, value)
		}
	})
}

func TestCodespaceActionErrorResponses(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		interaction bool
		status      int
		redirect    bool
	}{
		{"invalid interaction", codespace_service.ErrInteractionInvalidArgument, true, http.StatusSeeOther, true},
		{"missing interaction", codespace_service.ErrInteractionNotFound, true, http.StatusNotFound, false},
		{"denied interaction", codespace_service.ErrInteractionPermissionDenied, true, http.StatusNotFound, false},
		{"unavailable interaction", codespace_service.ErrInteractionStateUnavailable, true, http.StatusSeeOther, true},
		{"exhausted interaction", codespace_service.ErrInteractionVersionExhausted, true, http.StatusSeeOther, true},
		{"missing lifecycle", codespace_service.ErrLifecycleActionNotFound, false, http.StatusNotFound, false},
		{"denied lifecycle", codespace_service.ErrLifecycleActionPermissionDenied, false, http.StatusNotFound, false},
		{"unavailable lifecycle", codespace_service.ErrLifecycleActionStateUnavailable, false, http.StatusSeeOther, true},
		{"exhausted lifecycle", codespace_service.ErrLifecycleActionVersionExhausted, false, http.StatusSeeOther, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, resp := contexttest.MockContext(t, "POST /-/codespaces/test/action")
			ctx.SetPathParam("uuid", "test")
			if test.interaction {
				handleInteractionError(ctx, "TestAction", test.err, "/-/codespaces/test")
			} else {
				handleLifecycleActionError(ctx, "TestAction", test.err, "/-/codespaces/test")
			}
			assert.Equal(t, test.status, resp.Code)
			if test.redirect {
				assert.Equal(t, "/-/codespaces/test", resp.Header().Get("Location"))
			}
		})
	}
}
