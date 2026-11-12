// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/timeutil"
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

func TestGetRepoRunWorkflowIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const (
		repoID             = int64(4)
		repoWorkflowID     = "repo-orphan.yaml"
		scopedWorkflowID   = "scoped-only.yaml"
		sharedWorkflowID   = "shared-name.yaml"
		scopedWorkflowRepo = int64(111)
	)
	for _, spec := range []struct {
		id             int64
		workflowID     string
		workflowRepoID int64
		isScopedRun    bool
	}{
		{99811, repoWorkflowID, repoID, false},
		{99812, scopedWorkflowID, scopedWorkflowRepo, true},
		{99813, sharedWorkflowID, repoID, false},
		{99814, sharedWorkflowID, scopedWorkflowRepo, true},
	} {
		require.NoError(t, db.Insert(t.Context(), &ActionRun{
			ID:             spec.id,
			Index:          spec.id,
			RepoID:         repoID,
			OwnerID:        1,
			TriggerUserID:  1,
			WorkflowID:     spec.workflowID,
			WorkflowRepoID: spec.workflowRepoID,
			IsScopedRun:    spec.isScopedRun,
		}))
	}

	ids, err := GetRepoRunWorkflowIDs(t.Context(), repoID)
	require.NoError(t, err)
	assert.Contains(t, ids, repoWorkflowID)
	assert.Contains(t, ids, sharedWorkflowID)
	assert.NotContains(t, ids, scopedWorkflowID)
}

func TestGetStatusInfoList(t *testing.T) {
	statusInfoList := GetStatusInfoList(t.Context(), translation.MockLocale{})

	assert.Equal(t, []StatusInfo{
		{Status: int(StatusSuccess), StatusName: StatusSuccess.String(), DisplayedStatus: "actions.status.success"},
		{Status: int(StatusFailure), StatusName: StatusFailure.String(), DisplayedStatus: "actions.status.failure"},
		{Status: int(StatusCancelled), StatusName: StatusCancelled.String(), DisplayedStatus: "actions.status.cancelled"},
		{Status: int(StatusSkipped), StatusName: StatusSkipped.String(), DisplayedStatus: "actions.status.skipped"},
		{Status: int(StatusWaiting), StatusName: StatusWaiting.String(), DisplayedStatus: "actions.status.waiting"},
		{Status: int(StatusRunning), StatusName: StatusRunning.String(), DisplayedStatus: "actions.status.running"},
		{Status: int(StatusBlocked), StatusName: StatusBlocked.String(), DisplayedStatus: "actions.status.blocked"},
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

func TestFindRunOptionsSorting(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	const workflowID = "sort-test.yaml"
	for i, spec := range []struct {
		created timeutil.TimeStamp
		status  Status
		ref     string
	}{
		{300, StatusSuccess, "refs/heads/z"},
		{100, StatusFailure, "refs/heads/a"},
		{200, StatusFailure, "refs/heads/a"},
		{200, StatusRunning, "refs/tags/v1"},
		{100, StatusBlocked, "refs/heads/z"},
		{400, StatusUnknown, "refs/heads/a"},
	} {
		run := &ActionRun{ID: 99001 + int64(i), Index: 99001 + int64(i), RepoID: 4, OwnerID: 1, TriggerUserID: 1, WorkflowID: workflowID, Created: spec.created, Status: spec.status, Ref: spec.ref}
		_, err := db.GetEngine(t.Context()).NoAutoTime().Insert(run)
		require.NoError(t, err)
	}
	for _, tt := range []struct {
		sort string
		want []int64
	}{
		{"", []int64{99006, 99005, 99004, 99003, 99002, 99001}},
		{"newest", []int64{99006, 99001, 99004, 99003, 99005, 99002}},
		{"oldest", []int64{99002, 99005, 99003, 99004, 99001, 99006}},
		{"status", []int64{99003, 99002, 99005, 99004, 99001, 99006}},
		{"reverse_status", []int64{99006, 99001, 99004, 99005, 99003, 99002}},
		{"ref", []int64{99006, 99003, 99002, 99005, 99001, 99004}},
		{"reverse_ref", []int64{99004, 99005, 99001, 99006, 99003, 99002}},
		{"created; DROP TABLE action_run", []int64{99006, 99005, 99004, 99003, 99002, 99001}},
	} {
		t.Run(tt.sort, func(t *testing.T) {
			var ids []int64
			for page := 1; page <= 3; page++ {
				runs, total, err := db.FindAndCount[ActionRun](t.Context(), FindRunOptions{RepoID: 4, WorkflowID: workflowID, Sort: tt.sort, ListOptions: db.ListOptions{Page: page, PageSize: 2}})
				require.NoError(t, err)
				assert.EqualValues(t, 6, total)
				require.Len(t, runs, 2)
				for _, run := range runs {
					ids = append(ids, run.ID)
				}
			}
			assert.Equal(t, tt.want, ids)
		})
	}
	runs, total, err := db.FindAndCount[ActionRun](t.Context(), FindRunOptions{RepoID: 4, WorkflowID: workflowID, TriggerUserID: 1, Ref: "refs/heads/a", Status: []Status{StatusFailure}, Sort: "oldest"})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, runs, 2)
	assert.Equal(t, []int64{99002, 99003}, []int64{runs[0].ID, runs[1].ID})
	assert.Equal(t, "`action_run`.`index` DESC", (FindRunOptions{RepoID: 4}).ToOrders())
	assert.Equal(t, "`action_run`.`id` DESC", (FindRunOptions{}).ToOrders())
}

func TestFindRunOptionsStatusPriority(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	statuses := []Status{StatusFailure, StatusBlocked, StatusRunning, StatusCancelling, StatusWaiting, StatusPending, StatusCancelled, StatusSkipped, StatusSuccess, StatusUnknown}
	for i, status := range statuses {
		require.NoError(t, db.Insert(t.Context(), &ActionRun{ID: 99100 + int64(i), Index: 99100 + int64(i), RepoID: 4, WorkflowID: "status-sort.yaml", Status: status}))
	}
	runs, err := db.Find[ActionRun](t.Context(), FindRunOptions{RepoID: 4, WorkflowID: "status-sort.yaml", Sort: "status"})
	require.NoError(t, err)
	require.Len(t, runs, len(statuses))
	for i, run := range runs {
		assert.Equal(t, statuses[i], run.Status)
	}
}
