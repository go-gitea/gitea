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

		event := map[string]any{}
		assert.NoError(t, json.Unmarshal([]byte(updated), &event))
		assert.Equal(t, "*/5 * * * *", event["schedule"])
		assert.Equal(t, "refs/heads/main", event["ref"])
	})

	t.Run("keeps empty payload", func(t *testing.T) {
		updated := withScheduleInEventPayload("", "37 12 5 1 2")
		assert.Empty(t, updated)
	})

	t.Run("keeps payload when schedule empty", func(t *testing.T) {
		payload := `{"ref":"refs/heads/main"}`
		updated := withScheduleInEventPayload(payload, "")
		assert.Equal(t, payload, updated)
	})

	t.Run("keeps payload when malformed JSON", func(t *testing.T) {
		payload := `not a json object`
		updated := withScheduleInEventPayload(payload, "*/5 * * * *")
		assert.Equal(t, payload, updated)
	})
}
