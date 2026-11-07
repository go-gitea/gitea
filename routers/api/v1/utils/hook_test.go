// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"net/http"
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/structs"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestTestHookValidation(t *testing.T) {
	unittest.PrepareTestEnv(t)

	cases := []struct {
		name     string
		hookType string
		config   map[string]string
		status   int
	}{
		{"Test Validation", "gitea", map[string]string{"content_type": "json", "url": "https://example.com/webhook"}, 0}, // not written yet
		{"Test Validation with invalid URL", "gitea", map[string]string{"content_type": "json", "url": "example.com/webhook"}, http.StatusUnprocessableEntity},
		{"Test Validation with invalid webhook type", "unknown", map[string]string{"content_type": "json", "url": "example.com/webhook"}, http.StatusUnprocessableEntity},
		{"Test Validation with empty content type", "unknown", map[string]string{"url": "https://example.com/webhook"}, http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := contexttest.MockAPIContext(t, "user2/repo1/hooks")
			contexttest.LoadRepo(t, ctx, 1)
			contexttest.LoadRepoCommit(t, ctx)
			contexttest.LoadUser(t, ctx, 2)

			checkCreateHookOption(ctx, &structs.CreateHookOption{
				Type:   tc.hookType,
				Config: tc.config,
			})
			assert.Equal(t, tc.status, ctx.Resp.WrittenStatus())
		})
	}
}
