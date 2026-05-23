// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/translation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTriggerEvents(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	events, err := GetTriggerEvents(t.Context(), 4)
	require.NoError(t, err)
	assert.Equal(t, []string{"push"}, events)

	events, err = GetTriggerEvents(t.Context(), 5)
	require.NoError(t, err)
	assert.Equal(t, []string{"schedule"}, events)

	events, err = GetTriggerEvents(t.Context(), 999999)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestGetRunBranches(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	branches, err := GetRunBranches(t.Context(), 4)
	require.NoError(t, err)
	assert.Equal(t, []string{"master", "test"}, branches)

	branches, err = GetRunBranches(t.Context(), 999999)
	require.NoError(t, err)
	assert.Empty(t, branches)
}

func TestGetTriggerEventInfoList(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	eventInfoList, err := GetTriggerEventInfoList(t.Context(), translation.NewLocale("en-US"), 5)
	require.NoError(t, err)
	require.Len(t, eventInfoList, 1)
	assert.Equal(t, "schedule", eventInfoList[0].Event)
	assert.NotEmpty(t, eventInfoList[0].Display)
}

func TestGetTriggerEventsAndBranchesWithInsertedRuns(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	run := &ActionRun{
		Title:         "filter dropdown test run",
		RepoID:        1,
		OwnerID:       2,
		WorkflowID:    "filter-test.yaml",
		Index:         99998,
		TriggerUserID: 2,
		Ref:           "refs/heads/feature/filter",
		CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:         "push",
		TriggerEvent:  "workflow_dispatch",
		Status:        StatusSuccess,
	}
	require.NoError(t, db.Insert(t.Context(), run))
	defer func() {
		_, err := db.DeleteByID[ActionRun](t.Context(), run.ID)
		require.NoError(t, err)
	}()

	events, err := GetTriggerEvents(t.Context(), 1)
	require.NoError(t, err)
	assert.Contains(t, events, "workflow_dispatch")

	branches, err := GetRunBranches(t.Context(), 1)
	require.NoError(t, err)
	assert.Contains(t, branches, "feature/filter")
}
