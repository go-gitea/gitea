// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithScheduleInEventPayload(t *testing.T) {
	t.Run("adds schedule to existing payload", func(t *testing.T) {
		payload := `{"ref":"refs/heads/main"}`
		updated := withScheduleInEventPayload(payload, "*/5 * * * *", nil)

		event := map[string]any{}
		assert.NoError(t, json.Unmarshal([]byte(updated), &event))
		assert.Equal(t, "*/5 * * * *", event["schedule"])
		assert.Equal(t, "refs/heads/main", event["ref"])
	})

	t.Run("adds schedule to null payload", func(t *testing.T) {
		updated := withScheduleInEventPayload("null", "37 12 5 1 2", nil)

		event := map[string]any{}
		assert.NoError(t, json.Unmarshal([]byte(updated), &event))
		assert.Equal(t, "37 12 5 1 2", event["schedule"])
	})

	t.Run("adds schedule to empty payload", func(t *testing.T) {
		updated := withScheduleInEventPayload("", "37 12 5 1 2", nil)

		event := map[string]any{}
		assert.NoError(t, json.Unmarshal([]byte(updated), &event))
		assert.Equal(t, "37 12 5 1 2", event["schedule"])
	})

	t.Run("adds schedule with repository, sender, organization", func(t *testing.T) {
		updated := withScheduleInEventPayload("null", "@weekly", map[string]any{
			"repository":   &api.Repository{Name: "test-repo"},
			"sender":       &api.User{UserName: "test-user"},
			"organization": &api.Organization{Name: "test-org"},
		})

		event := map[string]any{}
		assert.NoError(t, json.Unmarshal([]byte(updated), &event))
		assert.Equal(t, "@weekly", event["schedule"])
		repository, ok := event["repository"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "test-repo", repository["name"])
		sender, ok := event["sender"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "test-user", sender["login"])
		organization, ok := event["organization"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "test-org", organization["name"])
	})

	t.Run("keeps payload when schedule empty", func(t *testing.T) {
		payload := `{"ref":"refs/heads/main"}`
		updated := withScheduleInEventPayload(payload, "", nil)
		assert.Equal(t, payload, updated)
	})

	t.Run("keeps payload when malformed JSON", func(t *testing.T) {
		payload := `not a json object`
		updated := withScheduleInEventPayload(payload, "*/5 * * * *", nil)
		assert.Equal(t, payload, updated)
	})
}
