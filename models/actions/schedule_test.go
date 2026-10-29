// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"testing"
	"time"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceScheduleTasksByRepo(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// no fixture covers these tables, so leave them as they were found
	const repoID = 4711
	t.Cleanup(func() {
		require.NoError(t, DeleteScheduleTaskByRepo(context.Background(), repoID))
	})

	registered := func(spec string) []*ActionSchedule {
		return []*ActionSchedule{{
			RepoID:     repoID,
			OwnerID:    5,
			WorkflowID: "nightly.yml",
			Specs:      []string{spec},
		}}
	}

	require.NoError(t, ReplaceScheduleTasksByRepo(t.Context(), repoID, registered("17 */6 * * *")))
	scheduleID := unittest.AssertExistsAndLoadBean(t, &ActionSchedule{RepoID: repoID}).ID

	// the occurrence came due while the scheduler had not made its pass yet
	due := timeutil.TimeStamp(time.Now().Add(-time.Minute).Unix())
	spec := unittest.AssertExistsAndLoadBean(t, &ActionScheduleSpec{RepoID: repoID})
	spec.Next = due
	require.NoError(t, UpdateScheduleSpec(t.Context(), spec, "next"))

	// an unrelated push re-registers the unchanged workflow, which must not consume that occurrence
	require.NoError(t, ReplaceScheduleTasksByRepo(t.Context(), repoID, registered("17 */6 * * *")))
	assert.Equal(t, 1, unittest.GetCount(t, &ActionScheduleSpec{RepoID: repoID}))
	kept := unittest.AssertExistsAndLoadBean(t, &ActionScheduleSpec{RepoID: repoID})
	assert.Equal(t, spec.ID, kept.ID, "the scheduler's own row should survive the replacement")
	assert.Equal(t, due, kept.Next)

	// and it must follow the new schedule row, so the run picks up the pushed commit and content
	reregistered := unittest.AssertExistsAndLoadBean(t, &ActionSchedule{RepoID: repoID})
	assert.NotEqual(t, scheduleID, reregistered.ID)
	assert.Equal(t, reregistered.ID, kept.ScheduleID)

	// a cron that is no longer registered has no row to keep, so it starts from the push
	require.NoError(t, ReplaceScheduleTasksByRepo(t.Context(), repoID, registered("23 */6 * * *")))
	assert.Equal(t, 1, unittest.GetCount(t, &ActionScheduleSpec{RepoID: repoID}))
	assert.Greater(t, unittest.AssertExistsAndLoadBean(t, &ActionScheduleSpec{RepoID: repoID}).Next, due)
}
