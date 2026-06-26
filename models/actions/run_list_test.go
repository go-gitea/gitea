// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/translation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRunWorkflowIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ids, err := GetRunWorkflowIDs(t.Context(), 4)
	assert.NoError(t, err)
	assert.Equal(t, []string{"artifact.yaml", "test.yaml"}, ids)

	ids, err = GetRunWorkflowIDs(t.Context(), 999999)
	assert.NoError(t, err)
	assert.Empty(t, ids)
}

func TestGetStatusInfoList(t *testing.T) {
	statusInfoList := GetStatusInfoList(t.Context(), translation.MockLocale{})

	assert.Equal(t, []StatusInfo{
		{Status: int(StatusSuccess), StatusName: StatusSuccess.String(), DisplayedStatus: "actions.status.success"},
		{Status: int(StatusFailure), StatusName: StatusFailure.String(), DisplayedStatus: "actions.status.failure"},
		{Status: int(StatusWaiting), StatusName: StatusWaiting.String(), DisplayedStatus: "actions.status.waiting"},
		{Status: int(StatusRunning), StatusName: StatusRunning.String(), DisplayedStatus: "actions.status.running"},
		{Status: int(StatusCancelling), StatusName: StatusCancelling.String(), DisplayedStatus: "actions.status.cancelling"},
	}, statusInfoList)
}

// TestFindRunOptions_WorkflowRepoID: two runs share the bare WorkflowID but come from different content-source repos;
// the source-aware WorkflowRepoID filter must separate them.
func TestFindRunOptions_WorkflowRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const (
		repoID     = int64(4)
		sourceA    = int64(111)
		sourceB    = int64(222)
		workflowID = "u3-shared.yaml"
	)
	for _, spec := range []struct{ id, workflowRepoID int64 }{
		{99801, sourceA},
		{99802, sourceB},
	} {
		require.NoError(t, db.Insert(t.Context(), &ActionRun{
			ID:             spec.id,
			Index:          spec.id,
			RepoID:         repoID,
			OwnerID:        1,
			TriggerUserID:  1,
			WorkflowID:     workflowID,
			WorkflowRepoID: spec.workflowRepoID,
			IsScopedRun:    true,
		}))
	}

	// no source filter -> both
	all, err := db.Find[ActionRun](t.Context(), FindRunOptions{RepoID: repoID, WorkflowID: workflowID})
	require.NoError(t, err)
	assert.Len(t, all, 2)

	// filter by source A -> only the run whose content came from A
	onlyA, err := db.Find[ActionRun](t.Context(), FindRunOptions{RepoID: repoID, WorkflowID: workflowID, WorkflowRepoID: sourceA})
	require.NoError(t, err)
	require.Len(t, onlyA, 1)
	assert.EqualValues(t, 99801, onlyA[0].ID)

	// filter by source B -> only the run whose content came from B
	onlyB, err := db.Find[ActionRun](t.Context(), FindRunOptions{RepoID: repoID, WorkflowID: workflowID, WorkflowRepoID: sourceB})
	require.NoError(t, err)
	require.Len(t, onlyB, 1)
	assert.EqualValues(t, 99802, onlyB[0].ID)
}

func TestFindRunOptions_IsScopedRun(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const (
		repoID     = int64(4)
		workflowID = "scoped-flag.yaml"
	)
	for _, spec := range []struct {
		id     int64
		scoped bool
	}{
		{99821, false},
		{99822, true},
	} {
		require.NoError(t, db.Insert(t.Context(), &ActionRun{
			ID:             spec.id,
			Index:          spec.id,
			RepoID:         repoID,
			OwnerID:        1,
			TriggerUserID:  1,
			WorkflowID:     workflowID,
			WorkflowRepoID: repoID,
			IsScopedRun:    spec.scoped,
		}))
	}

	repoLevel, err := db.Find[ActionRun](t.Context(), FindRunOptions{RepoID: repoID, WorkflowID: workflowID, IsScopedRun: optional.Some(false)})
	require.NoError(t, err)
	require.Len(t, repoLevel, 1)
	assert.EqualValues(t, 99821, repoLevel[0].ID)

	scoped, err := db.Find[ActionRun](t.Context(), FindRunOptions{RepoID: repoID, WorkflowID: workflowID, IsScopedRun: optional.Some(true)})
	require.NoError(t, err)
	require.Len(t, scoped, 1)
	assert.EqualValues(t, 99822, scoped[0].ID)
}
