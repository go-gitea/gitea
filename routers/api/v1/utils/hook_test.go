// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"net/http"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/models/webhook"
	"gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestEditHook(t *testing.T) {
	unittest.PrepareTestEnv(t)

	hook := &webhook.Webhook{
		RepoID:      1,
		URL:         "https://example.com/webhook",
		ContentType: webhook.ContentTypeJSON,
		IsActive:    true,
		HookEvent: &webhook_module.HookEvent{
			BranchFilter: "main",
			ChooseEvents: true,
			HookEvents:   webhook_module.HookEvents{webhook_module.HookEventIssues: true},
		},
	}
	require.NoError(t, hook.SetHeaderAuthorization("Bearer secret"))
	require.NoError(t, hook.UpdateEvent())
	require.NoError(t, db.Insert(t.Context(), hook))

	t.Run("KeepsOmittedFields", func(t *testing.T) {
		ctx, _ := contexttest.MockAPIContext(t, "user2/repo1/hooks")
		assert.True(t, editHook(ctx, &structs.EditHookOption{
			Active: new(false),
			Events: []string{},
			Config: map[string]string{"url": hook.URL},
		}, hook))

		updated := unittest.AssertExistsAndLoadBean(t, &webhook.Webhook{ID: hook.ID})
		assert.False(t, updated.IsActive)
		assert.Equal(t, "main", updated.BranchFilter)
		assert.True(t, updated.HasEvent(webhook_module.HookEventIssues))
		assert.False(t, updated.HasEvent(webhook_module.HookEventPush))
		header, err := updated.HeaderAuthorization()
		require.NoError(t, err)
		assert.Equal(t, "Bearer secret", header)
	})

	t.Run("NewURLClearsAuthorizationHeader", func(t *testing.T) {
		ctx, _ := contexttest.MockAPIContext(t, "user2/repo1/hooks")
		assert.True(t, editHook(ctx, &structs.EditHookOption{Config: map[string]string{"url": "https://example.org/webhook"}}, hook))
		assert.Empty(t, unittest.AssertExistsAndLoadBean(t, &webhook.Webhook{ID: hook.ID}).HeaderAuthorizationEncrypted)
	})
}
