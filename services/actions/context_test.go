// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/unittest"

	act_model "github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateRunConcurrency_RunIDFallback(t *testing.T) {
	// Regression: two push-event runs evaluating
	//   ${{ github.workflow }}-${{ github.head_ref || github.run_id }}
	// must produce distinct concurrency groups. head_ref is empty on push,
	// so github.run_id is the only uniqueness source; if it evaluated to ""
	// (as it did before run.ID was populated pre-evaluation), both runs would
	// share a group and cancel-in-progress would cross-cancel unrelated
	// branches.
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	runA := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 791})
	runB := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 792})

	expr := &act_model.RawConcurrency{
		Group:            "${{ github.workflow }}-${{ github.head_ref || github.run_id }}",
		CancelInProgress: "true",
	}

	assert.NoError(t, EvaluateRunConcurrencyFillModel(ctx, runA, expr, nil, nil))
	assert.NoError(t, EvaluateRunConcurrencyFillModel(ctx, runB, expr, nil, nil))

	assert.Contains(t, runA.ConcurrencyGroup, "791")
	assert.Contains(t, runB.ConcurrencyGroup, "792")
	assert.NotEqual(t, runA.ConcurrencyGroup, runB.ConcurrencyGroup)
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
