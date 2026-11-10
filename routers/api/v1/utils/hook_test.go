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

func TestEditSystemHook(t *testing.T) {
	unittest.PrepareTestEnv(t)

	hook := &webhook.Webhook{
		Type:            webhook_module.SLACK,
		URL:             "https://example.com/webhook",
		ContentType:     webhook.ContentTypeJSON,
		IsActive:        true,
		IsSystemWebhook: true,
		Meta:            `{"channel":"#old","username":"bot"}`,
		HookEvent: &webhook_module.HookEvent{
			BranchFilter: "main",
			ChooseEvents: true,
			HookEvents:   webhook_module.HookEvents{webhook_module.HookEventIssues: true},
		},
	}
	require.NoError(t, hook.SetHeaderAuthorization("Bearer secret"))
	require.NoError(t, hook.UpdateEvent())
	require.NoError(t, db.Insert(t.Context(), hook))

	t.Run("UpdatesSentAndKeepsOmittedFields", func(t *testing.T) {
		ctx, resp := contexttest.MockAPIContext(t, "api/v1/admin/hooks")
		EditSystemHook(ctx, &structs.EditHookOption{
			Active: new(false),
			Events: []string{},
			Config: map[string]string{"url": hook.URL, "channel": "#new", "secret": "new-secret"},
		}, hook.ID)
		assert.Equal(t, http.StatusOK, resp.Code)

		updated := unittest.AssertExistsAndLoadBean(t, &webhook.Webhook{ID: hook.ID})
		assert.Equal(t, "new-secret", updated.Secret)
		assert.False(t, updated.IsActive)
		assert.Equal(t, "main", updated.BranchFilter)
		assert.True(t, updated.HasEvent(webhook_module.HookEventIssues))
		assert.False(t, updated.HasEvent(webhook_module.HookEventPush))
		assert.JSONEq(t, `{"channel":"#new","username":"bot","icon_url":"","color":""}`, updated.Meta)
		header, err := updated.HeaderAuthorization()
		require.NoError(t, err)
		assert.Equal(t, "Bearer secret", header)
	})

	t.Run("NewURLClearsAuthorizationHeader", func(t *testing.T) {
		ctx, resp := contexttest.MockAPIContext(t, "api/v1/admin/hooks")
		EditSystemHook(ctx, &structs.EditHookOption{Config: map[string]string{"url": "https://example.org/webhook"}}, hook.ID)
		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Empty(t, unittest.AssertExistsAndLoadBean(t, &webhook.Webhook{ID: hook.ID}).HeaderAuthorizationEncrypted)
	})

	t.Run("InvalidURLAsAdmin", func(t *testing.T) {
		ctx, resp := contexttest.MockAPIContext(t, "api/v1/admin/hooks")
		contexttest.LoadUser(t, ctx, 1)
		EditSystemHook(ctx, &structs.EditHookOption{Config: map[string]string{"url": "not a url"}}, hook.ID)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("NotFound", func(t *testing.T) {
		ctx, resp := contexttest.MockAPIContext(t, "api/v1/admin/hooks")
		EditSystemHook(ctx, &structs.EditHookOption{}, 999999)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})
}
