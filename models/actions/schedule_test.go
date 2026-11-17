// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceScheduleTasksByRepo(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	const repoID = 4711
	register := func(commitSHA, spec string) {
		require.NoError(t, ReplaceScheduleTasksByRepo(t.Context(), repoID, []*ActionSchedule{
			{RepoID: repoID, WorkflowID: "nightly.yml", CommitSHA: commitSHA, Specs: []string{spec}},
		}))
	}

	register("c1", "17 */6 * * *")
	spec := unittest.AssertExistsAndLoadBean(t, &ActionScheduleSpec{RepoID: repoID})
	spec.Next = timeutil.TimeStamp(time.Now().Add(-time.Minute).Unix())
	require.NoError(t, UpdateScheduleSpec(t.Context(), spec, "next"))

	register("c2", "17 */6 * * *")
	unittest.AssertExistsAndLoadBean(t, &ActionSchedule{ID: spec.ScheduleID, CommitSHA: "c2"})
	unittest.AssertExistsAndLoadBean(t, &ActionScheduleSpec{ID: spec.ID, ScheduleID: spec.ScheduleID, Next: spec.Next})

	register("c3", "23 */6 * * *")
	unittest.AssertNotExistsBean(t, &ActionScheduleSpec{ID: spec.ID})
	assert.Greater(t, unittest.AssertExistsAndLoadBean(t, &ActionScheduleSpec{RepoID: repoID}).Next, spec.Next)

	require.NoError(t, ReplaceScheduleTasksByRepo(t.Context(), repoID, nil))
	unittest.AssertNotExistsBean(t, &ActionSchedule{RepoID: repoID})
	unittest.AssertNotExistsBean(t, &ActionScheduleSpec{RepoID: repoID})
}
