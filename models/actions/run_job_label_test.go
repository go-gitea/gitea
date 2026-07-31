// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

// jobLabelsMatchRunner is the Go equivalent of runnerMatchableJobCond: every
// required job label must be covered by the runner's label set. An empty job
// runs_on matches any runner that has at least one label; a runner with no
// labels matches only jobs that require no label.
func jobLabelsMatchRunner(jobLabels, runnerLabels []string) bool {
	if len(jobLabels) == 0 {
		return true
	}
	if len(runnerLabels) == 0 {
		return false
	}
	return container.SetOf(runnerLabels...).Contains(jobLabels...)
}

func TestRunnerLabelMatchingContract(t *testing.T) {
	cases := []struct {
		name    string
		runner  []string
		job     []string
		matches bool
	}{
		{name: "runner covers all job labels", runner: []string{"ubuntu-latest", "x64"}, job: []string{"ubuntu-latest"}, matches: true},
		{name: "runner missing a job label", runner: []string{"ubuntu-latest"}, job: []string{"ubuntu-latest", "self-hosted"}, matches: false},
		{name: "empty job runs_on matches labeled runner", runner: []string{"ubuntu-latest"}, job: nil, matches: true},
		{name: "empty job runs_on matches unlabeled runner", runner: nil, job: nil, matches: true},
		{name: "labeled job does not match unlabeled runner", runner: nil, job: []string{"ubuntu-latest"}, matches: false},
		{name: "case-sensitive mismatch", runner: []string{"Linux"}, job: []string{"linux"}, matches: false},
		{name: "duplicate job labels", runner: []string{"a", "b"}, job: []string{"a", "a", "b"}, matches: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &ActionRunner{AgentLabels: tc.runner}
			assert.Equal(t, tc.matches, runner.CanMatchLabels(tc.job), "CanMatchLabels")
			assert.Equal(t, tc.matches, jobLabelsMatchRunner(tc.job, tc.runner), "jobLabelsMatchRunner")
		})
	}
}

func TestRunnerMatchableJobCondSQL(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	run := &ActionRun{
		Title: "label-cond-test", RepoID: 1, OwnerID: 2, WorkflowID: "test.yaml",
		Index: 9001, TriggerUserID: 2, Ref: "refs/heads/master",
		CommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:     "push", TriggerEvent: "push", Status: StatusWaiting,
	}
	require.NoError(t, db.Insert(t.Context(), run))

	payload := []byte(`name: test
on: push
jobs:
  j:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
`)

	cases := []struct {
		name    string
		runner  []string
		jobName string
		runsOn  []string
		matches bool
	}{
		{name: "sql matches labeled runner", runner: []string{"ubuntu-latest"}, jobName: "match", runsOn: []string{"ubuntu-latest"}, matches: true},
		{name: "sql rejects missing label", runner: []string{"ubuntu-latest"}, jobName: "nomatch", runsOn: []string{"macos"}, matches: false},
		{name: "sql empty runs_on", runner: []string{"ubuntu-latest"}, jobName: "empty", runsOn: nil, matches: true},
		{name: "sql unlabeled runner", runner: nil, jobName: "unlabeled-only", runsOn: nil, matches: true},
		{name: "sql unlabeled runner vs labeled job", runner: nil, jobName: "needs-label", runsOn: []string{"ubuntu-latest"}, matches: false},
	}

	jobIDs := make(map[string]int64)
	for _, tc := range cases {
		job := &ActionRunJob{
			RunID: run.ID, RepoID: 1, OwnerID: 2,
			CommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0",
			Name:      tc.jobName, Attempt: 1, JobID: tc.jobName,
			RunsOn: tc.runsOn, Status: StatusWaiting, WorkflowPayload: payload,
		}
		require.NoError(t, InsertActionRunJob(t.Context(), job))
		jobIDs[tc.jobName] = job.ID
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &ActionRunner{AgentLabels: tc.runner}
			assert.Equal(t, tc.matches, runner.CanMatchLabels(tc.runsOn))

			matchCond := runnerMatchableJobCond(tc.runner)
			has, err := db.GetEngine(t.Context()).
				Table("action_run_job").
				Where(builder.Eq{"id": jobIDs[tc.jobName]}).
				And(matchCond).
				Exist()
			require.NoError(t, err)
			assert.Equal(t, tc.matches, has, "runnerMatchableJobCond SQL")
		})
	}
}
