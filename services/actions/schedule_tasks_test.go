// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
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
		assert.Equal(t, "test-repo", event["repository"].(map[string]any)["name"])
		assert.Equal(t, "test-user", event["sender"].(map[string]any)["login"])
		assert.Equal(t, "test-org", event["organization"].(map[string]any)["name"])
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
