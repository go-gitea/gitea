// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"
	webhook_module "gitea.dev/modules/webhook"

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

func TestStartTasks(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	insertSchedule := func(repoID, ownerID int64, workflowID, cronSpec, content string, next timeutil.TimeStamp) *actions_model.ActionScheduleSpec {
		schedule := &actions_model.ActionSchedule{
			Title:         workflowID,
			RepoID:        repoID,
			OwnerID:       ownerID,
			WorkflowID:    workflowID,
			TriggerUserID: 1,
			Ref:           "refs/heads/master",
			CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
			Event:         webhook_module.HookEventSchedule,
			EventPayload:  "{}",
			Content:       []byte(content),
		}
		require.NoError(t, db.Insert(t.Context(), schedule))
		spec := &actions_model.ActionScheduleSpec{RepoID: repoID, ScheduleID: schedule.ID, Spec: cronSpec, Next: next}
		require.NoError(t, db.Insert(t.Context(), spec))
		return spec
	}

	due := timeutil.TimeStamp(time.Now().Add(-time.Minute).Unix())
	validWorkflow := "jobs:\n  job:\n    runs-on: ubuntu-latest\n    steps:\n      - run: true\n"

	// specs are processed by ascending id, so the broken one runs first and used to abort the whole pass
	broken := insertSchedule(1, 2, "broken.yml", "@every 1m", "this: [is: not: a: workflow", due)
	valid := insertSchedule(4, 5, "valid.yml", "@every 1m", validWorkflow, due)
	never := insertSchedule(4, 5, "never.yml", "0 0 30 2 *", validWorkflow, timeutil.TimeStamp(time.Time{}.Unix()))

	require.ErrorContains(t, startTasks(t.Context()), "1 schedule(s) could not be started")

	assert.Equal(t, 1, unittest.GetCount(t, &actions_model.ActionRun{RepoID: 4, WorkflowID: "valid.yml"}))
	assert.Equal(t, 0, unittest.GetCount(t, &actions_model.ActionRun{RepoID: 1, WorkflowID: "broken.yml"}))
	assert.Equal(t, 0, unittest.GetCount(t, &actions_model.ActionRun{RepoID: 4, WorkflowID: "never.yml"}))

	// the broken spec moves on too, so it does not fail again on every pass
	for _, spec := range []*actions_model.ActionScheduleSpec{broken, valid} {
		updated := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionScheduleSpec{ID: spec.ID})
		assert.Greater(t, updated.Next, spec.Next)
	}
	assert.Equal(t, never.Next, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionScheduleSpec{ID: never.ID}).Next)
}
