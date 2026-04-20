// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
)

func TestWithScheduleInEventPayload(t *testing.T) {
	t.Run("adds schedule to existing payload", func(t *testing.T) {
		payload := `{"ref":"refs/heads/main"}`
		updated := withScheduleInEventPayload(payload, "*/5 * * * *")
		assert.NotEmpty(t, updated)

		event, err := eventPayloadAsMap(updated)
		assert.NoError(t, err)
		assert.Equal(t, "*/5 * * * *", event["schedule"])
		assert.Equal(t, "refs/heads/main", event["ref"])
	})

	t.Run("creates payload when empty", func(t *testing.T) {
		updated := withScheduleInEventPayload("", "37 12 5 1 2")
		assert.Empty(t, updated)
	})

	t.Run("keeps payload when schedule empty", func(t *testing.T) {
		payload := `{"ref":"refs/heads/main"}`
		updated := withScheduleInEventPayload(payload, "")
		assert.NotEmpty(t, updated)
		assert.Equal(t, payload, updated)
	})
}

func eventPayloadAsMap(payload string) (map[string]any, error) {
	event := map[string]any{}
	if payload == "" {
		return event, nil
	}
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return nil, err
	}
	return event, nil
}
