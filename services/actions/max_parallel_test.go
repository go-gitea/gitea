// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMaxParallel(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"3", 3},
		{"0", 0},
		{"-1", 0},
		{"1.5", 1},           // GitHub casts the YAML number to int
		{"-1.5", 0},          // truncates to -1, which means unlimited
		{"1e3", 256},         // clamped to MaxJobNumPerRun
		{"nan", 0},           // must not reach the int cast
		{"${{ vars.n }}", 0}, // expressions are not evaluated yet
		{"abc", 0},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, parseMaxParallel("job", tt.input), "input %q", tt.input)
	}
}

// a 5-job matrix capped at 2, so 2 jobs start and 3 stay blocked
const maxParallelWorkflow = `name: max-parallel
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      max-parallel: 2
      matrix:
        version: [1, 2, 3, 4, 5]
    steps:
      - run: echo hi
`

func insertMaxParallelRun(t *testing.T, needApproval bool) *actions_model.ActionRun {
	t.Helper()
	run := &actions_model.ActionRun{
		Title:             "max-parallel",
		RepoID:            4,
		OwnerID:           1,
		WorkflowID:        "max-parallel.yaml",
		TriggerUserID:     1,
		Ref:               "refs/heads/master",
		CommitSHA:         "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:             "push",
		TriggerEvent:      "push",
		EventPayload:      "{}",
		NeedApproval:      needApproval,
		WorkflowRepoID:    4,
		WorkflowCommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0",
	}
	require.NoError(t, PrepareRunAndInsert(t.Context(), []byte(maxParallelWorkflow), run, nil))
	return run
}

func runJobs(t *testing.T, runID, attemptID int64) actions_model.ActionJobList {
	t.Helper()
	jobs, err := actions_model.GetRunJobsByRunAndAttemptID(t.Context(), runID, attemptID)
	require.NoError(t, err)
	return jobs
}

func statusCounts(jobs actions_model.ActionJobList) map[actions_model.Status]int {
	counts := map[actions_model.Status]int{}
	for _, job := range jobs {
		counts[job.Status]++
	}
	return counts
}

func TestPrepareRunAndInsert_MaxParallel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	run := insertMaxParallelRun(t, false)
	jobs := runJobs(t, run.ID, run.LatestAttemptID)
	assert.Equal(t, map[actions_model.Status]int{
		actions_model.StatusWaiting: 2,
		actions_model.StatusBlocked: 3,
	}, statusCounts(jobs))

	for _, job := range jobs {
		assert.Equal(t, 2, job.MaxParallel, "every matrix sibling carries the limit")
	}
}
