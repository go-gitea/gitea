// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestRunIDForContext(t *testing.T) {
	// Regression: workflow-level concurrency is evaluated before the run is
	// inserted (run.ID == 0), so github.run_id must fall back to run.Index —
	// otherwise ${{ github.head_ref || github.run_id }} collapses to the same
	// string across all push events, cancelling runs across unrelated branches.
	assert.Equal(t, "42", runIDForContext(&actions_model.ActionRun{ID: 42, Index: 7}))
	assert.Equal(t, "7", runIDForContext(&actions_model.ActionRun{ID: 0, Index: 7}))
	assert.Empty(t, runIDForContext(&actions_model.ActionRun{ID: 0, Index: 0}))
}

func TestFindTaskNeeds(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 51})
	job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: task.JobID})

	ret, err := FindTaskNeeds(t.Context(), job)
	assert.NoError(t, err)
	assert.Len(t, ret, 1)
	assert.Contains(t, ret, "job1")
	assert.Len(t, ret["job1"].Outputs, 2)
	assert.Equal(t, "abc", ret["job1"].Outputs["output_a"])
	assert.Equal(t, "bbb", ret["job1"].Outputs["output_b"])
}
