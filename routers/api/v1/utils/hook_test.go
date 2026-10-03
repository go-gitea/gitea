// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"net/http"
	"strings"
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/models/webhook"
	"gitea.dev/modules/json"
	"gitea.dev/modules/structs"
	"gitea.dev/services/contexttest"
	webhook_service "gitea.dev/services/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestHookValidation(t *testing.T) {
	unittest.PrepareTestEnv(t)

	t.Run("Test Validation", func(t *testing.T) {
		ctx, _ := contexttest.MockAPIContext(t, "user2/repo1/hooks")
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadRepoCommit(t, ctx)
		contexttest.LoadUser(t, ctx, 2)

		checkCreateHookOption(ctx, &structs.CreateHookOption{
			Type: "gitea",
			Config: map[string]string{
				"content_type": "json",
				"url":          "https://example.com/webhook",
			},
		})
		assert.Equal(t, 0, ctx.Resp.WrittenStatus()) // not written yet
	})

	t.Run("Test Validation with invalid URL", func(t *testing.T) {
		ctx, _ := contexttest.MockAPIContext(t, "user2/repo1/hooks")
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadRepoCommit(t, ctx)
		contexttest.LoadUser(t, ctx, 2)

		checkCreateHookOption(ctx, &structs.CreateHookOption{
			Type: "gitea",
			Config: map[string]string{
				"content_type": "json",
				"url":          "example.com/webhook",
			},
		})
		assert.Equal(t, http.StatusUnprocessableEntity, ctx.Resp.WrittenStatus())
	})

	t.Run("Test Validation with invalid webhook type", func(t *testing.T) {
		ctx, _ := contexttest.MockAPIContext(t, "user2/repo1/hooks")
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadRepoCommit(t, ctx)
		contexttest.LoadUser(t, ctx, 2)

		checkCreateHookOption(ctx, &structs.CreateHookOption{
			Type: "unknown",
			Config: map[string]string{
				"content_type": "json",
				"url":          "example.com/webhook",
			},
		})
		assert.Equal(t, http.StatusUnprocessableEntity, ctx.Resp.WrittenStatus())
	})

	t.Run("Test Validation with empty content type", func(t *testing.T) {
		ctx, _ := contexttest.MockAPIContext(t, "user2/repo1/hooks")
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadRepoCommit(t, ctx)
		contexttest.LoadUser(t, ctx, 2)

		checkCreateHookOption(ctx, &structs.CreateHookOption{
			Type: "unknown",
			Config: map[string]string{
				"url": "https://example.com/webhook",
			},
		})
		assert.Equal(t, http.StatusUnprocessableEntity, ctx.Resp.WrittenStatus())
	})
}

func TestFluxerHookValidation(t *testing.T) {
	unittest.PrepareTestEnv(t)
	for _, tc := range []struct {
		name, contentType, username, iconURL string
		valid                                bool
	}{
		{name: "defaults", contentType: "json", valid: true},
		{name: "overrides", contentType: "json", username: "Gitea", iconURL: "https://gitea.example/icon.png", valid: true},
		{name: "form", contentType: "form"},
		{name: "long username", contentType: "json", username: strings.Repeat("😀", 41)},
		{name: "invalid icon", contentType: "json", iconURL: "https://localhost/icon.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := contexttest.MockAPIContext(t, "user2/repo1/hooks")
			valid := checkCreateHookOption(ctx, &structs.CreateHookOption{Type: "fluxer", Config: map[string]string{"url": "https://chat.example/webhook", "content_type": tc.contentType, "username": tc.username, "icon_url": tc.iconURL}})
			assert.Equal(t, tc.valid, valid)
			if !tc.valid {
				assert.Equal(t, http.StatusUnprocessableEntity, ctx.Resp.WrittenStatus())
			}
		})
	}
}

func TestFluxerHookMetaPatch(t *testing.T) {
	original := &webhook.Webhook{Meta: `{"username":"Old","icon_url":"https://gitea.example/icon.png"}`}
	for _, tc := range []struct {
		config            map[string]string
		username, iconURL string
	}{
		{nil, "Old", "https://gitea.example/icon.png"},
		{map[string]string{"username": "New"}, "New", "https://gitea.example/icon.png"},
		{map[string]string{"icon_url": "https://gitea.example/new.png"}, "Old", "https://gitea.example/new.png"},
		{map[string]string{"username": ""}, "", "https://gitea.example/icon.png"},
		{map[string]string{"icon_url": ""}, "Old", ""},
	} {
		encoded, err := fluxerHookMeta(original, tc.config)
		require.NoError(t, err)
		var meta webhook_service.FluxerMeta
		require.NoError(t, json.Unmarshal([]byte(encoded), &meta))
		assert.Equal(t, tc.username, meta.Username)
		assert.Equal(t, tc.iconURL, meta.IconURL)
	}
	assert.JSONEq(t, `{"username":"Old","icon_url":"https://gitea.example/icon.png"}`, original.Meta)
}
