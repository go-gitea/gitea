// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"net/http"
	"testing"

	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestCodespaceActionHelpers(t *testing.T) {
	t.Run("return path", func(t *testing.T) {
		tests := []struct {
			input string
			want  string
		}{
			{"", "/-/codespaces"},
			{"/-/codespaces?state=deleted", "/-/codespaces?state=deleted"},
			{"https://example.com/", "/-/codespaces"},
			{"//example.com/", "/-/codespaces"},
			{"relative", "/-/codespaces"},
		}
		for _, test := range tests {
			assert.Equal(t, test.want, validCodespaceReturnTo(test.input))
		}
	})

	t.Run("optional integer form", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "POST /-/codespaces/test/auto-stop")
		value, ok := parseOptionalTimeoutSecondsForm(ctx, 30)
		assert.True(t, ok)
		assert.EqualValues(t, 30, value)

		ctx.Req.Form.Set("timeout_seconds", "600")
		value, ok = parseOptionalTimeoutSecondsForm(ctx, 30)
		assert.True(t, ok)
		assert.EqualValues(t, 600, value)

		ctx.Req.Form.Set("timeout_seconds", "invalid")
		_, ok = parseOptionalTimeoutSecondsForm(ctx, 30)
		assert.False(t, ok)
	})
}

func TestCodespaceActionErrorResponses(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		handler func(*context.Context, string, error)
		status  int
		body    string
	}{
		{"invalid interaction", codespace_service.ErrInteractionInvalidArgument, handleInteractionError, http.StatusBadRequest, "invalid_argument"},
		{"missing interaction", codespace_service.ErrInteractionNotFound, handleInteractionError, http.StatusNotFound, ""},
		{"denied interaction", codespace_service.ErrInteractionPermissionDenied, handleInteractionError, http.StatusForbidden, "permission_denied"},
		{"unavailable interaction", codespace_service.ErrInteractionStateUnavailable, handleInteractionError, http.StatusConflict, "state_unavailable"},
		{"exhausted interaction", codespace_service.ErrInteractionVersionExhausted, handleInteractionError, http.StatusConflict, "version_exhausted"},
		{"missing lifecycle", codespace_service.ErrLifecycleActionNotFound, handleLifecycleActionError, http.StatusNotFound, ""},
		{"denied lifecycle", codespace_service.ErrLifecycleActionPermissionDenied, handleLifecycleActionError, http.StatusForbidden, "permission_denied"},
		{"unavailable lifecycle", codespace_service.ErrLifecycleActionStateUnavailable, handleLifecycleActionError, http.StatusConflict, "state_unavailable"},
		{"exhausted lifecycle", codespace_service.ErrLifecycleActionVersionExhausted, handleLifecycleActionError, http.StatusConflict, "version_exhausted"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, resp := contexttest.MockContext(t, "POST /-/codespaces/test/action")
			test.handler(ctx, "TestAction", test.err)
			assert.Equal(t, test.status, resp.Code)
			if test.body != "" {
				assert.Contains(t, resp.Body.String(), test.body)
			}
		})
	}
}
