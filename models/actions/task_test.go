// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strings"
	"testing"

	runnerv1 "gitea.dev/actions-proto-go/runner/v1"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	"xorm.io/builder"
)

func TestActionTask_GetRunJobLink(t *testing.T) {
	repo := &repo_model.Repository{OwnerName: "org", Name: "consumer"}
	run := &ActionRun{ID: 10, Repo: repo}
	job := &ActionRunJob{ID: 42, Run: run}

	// a task with a loaded job links to that specific job, not just the run
	task := &ActionTask{Job: job}
	assert.Equal(t, run.Link()+"/jobs/42", task.GetRunJobLink())

	// missing job, run or repo yields an empty link instead of a broken URL
	assert.Empty(t, (&ActionTask{}).GetRunJobLink())
	assert.Empty(t, (&ActionTask{Job: &ActionRunJob{ID: 42}}).GetRunJobLink())
	assert.Empty(t, (&ActionTask{Job: &ActionRunJob{ID: 42, Run: &ActionRun{ID: 10}}}).GetRunJobLink())
}

func TestMakeTaskStepDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		jobStep  *jobparser.Step
		expected string
	}{
		{
			name: "explicit name",
			jobStep: &jobparser.Step{
				Name: "Test Step",
			},
			expected: "Test Step",
		},
		{
			name: "uses step",
			jobStep: &jobparser.Step{
				Uses: "actions/checkout@v4",
			},
			expected: "Run actions/checkout@v4",
		},
		{
			name: "single-line run",
			jobStep: &jobparser.Step{
				Run: "echo hello",
			},
			expected: "Run echo hello",
		},
		{
			name: "multi-line run block scalar",
			jobStep: &jobparser.Step{
				Run: "\n  echo hello  \r\n  echo world  \n  ",
			},
			expected: "Run echo hello",
		},
		{
			name: "fallback to id",
			jobStep: &jobparser.Step{
				ID: "step-id",
			},
			expected: "Run step-id",
		},
		{
			name: "very long name truncated",
			jobStep: &jobparser.Step{
				Name: strings.Repeat("a", 300),
			},
			expected: strings.Repeat("a", 252) + "…",
		},
		{
			name: "very long run truncated",
			jobStep: &jobparser.Step{
				Run: strings.Repeat("a", 300),
			},
			expected: "Run " + strings.Repeat("a", 248) + "…",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := makeTaskStepDisplayName(tt.jobStep, 255)
			assert.Equal(t, tt.expected, result)
		})
	}
}

const (
	pickTestRepoID  = 1
	pickTestOwnerID = 2
)

var pickTestPayload = []byte(`name: test
on: push
jobs:
  j:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
`)

func newPickTestRun(t *testing.T, idx int64) *ActionRun {
	t.Helper()
	run := &ActionRun{
		Title: "pick-test-run", RepoID: pickTestRepoID, OwnerID: pickTestOwnerID, WorkflowID: "test.yaml",
		Index: idx, TriggerUserID: pickTestOwnerID, Ref: "refs/heads/master",
		CommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:     "push", TriggerEvent: "push", Status: StatusWaiting,
	}
	require.NoError(t, db.Insert(t.Context(), run))
	return run
}

// newPickTestWaitingJob inserts a waiting job and its labels via the production
// InsertActionRunJob path, so tests exercise the same label sync as real inserts.
func newPickTestWaitingJob(t *testing.T, runID int64, name string, runsOn []string, payload []byte) *ActionRunJob {
	t.Helper()
	job := &ActionRunJob{
		RunID: runID, RepoID: pickTestRepoID, OwnerID: pickTestOwnerID,
		CommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Name:      name, Attempt: 1, JobID: name,
		RunsOn: runsOn, Status: StatusWaiting, WorkflowPayload: payload,
	}
	require.NoError(t, InsertActionRunJob(t.Context(), job))
	return job
}

func TestCreateTaskForRunnerSkipsUnmatchableBacklog(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	run := newPickTestRun(t, 1000)

	// A head of older jobs this runner can never match, queued ahead of the
	// matchable job. The matchable job must not be starved behind them.
	for range 5 {
		newPickTestWaitingJob(t, run.ID, "unmatchable", []string{"macos-special"}, nil)
	}
	matchable := newPickTestWaitingJob(t, run.ID, "matchable", []string{"ubuntu-latest"}, pickTestPayload)

	runner := &ActionRunner{UUID: "runner-backlog-skip", Name: "runner-backlog-skip", RepoID: pickTestRepoID, AgentLabels: []string{"ubuntu-latest"}}
	require.NoError(t, db.Insert(t.Context(), runner))

	// Despite older non-matching jobs queued ahead of it, the matchable job must
	// be assigned rather than starved behind the unmatchable head.
	task, ok, err := CreateTaskForRunner(t.Context(), runner)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, task)
	assert.Equal(t, matchable.ID, task.JobID)
}

func TestCreateTaskForRunnerLabelMatching(t *testing.T) {
	t.Run("labeled runner matches job with empty runs_on", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		job := newPickTestWaitingJob(t, newPickTestRun(t, 2001).ID, "j", nil, pickTestPayload)
		runner := &ActionRunner{UUID: "r-empty-match", Name: "r-empty-match", RepoID: pickTestRepoID, AgentLabels: []string{"ubuntu-latest"}}
		require.NoError(t, db.Insert(t.Context(), runner))

		task, ok, err := CreateTaskForRunner(t.Context(), runner)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, job.ID, task.JobID)
	})

	t.Run("runner without labels matches only jobs requiring none", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		run := newPickTestRun(t, 2002)
		labeled := newPickTestWaitingJob(t, run.ID, "labeled", []string{"ubuntu-latest"}, pickTestPayload)
		unlabeled := newPickTestWaitingJob(t, run.ID, "unlabeled", nil, pickTestPayload)
		runner := &ActionRunner{UUID: "r-no-labels", Name: "r-no-labels", RepoID: pickTestRepoID}
		require.NoError(t, db.Insert(t.Context(), runner))

		task, ok, err := CreateTaskForRunner(t.Context(), runner)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, unlabeled.ID, task.JobID, "labeled job %d must not match a runner with no labels", labeled.ID)
	})
}

func TestCreateTaskForRunnerFailsUnpreparableJob(t *testing.T) {
	// A job whose payload won't parse must be failed and skipped, not abort the pick.
	t.Run("unparsable job is failed and a later job is still assigned", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		run := newPickTestRun(t, 3001)
		bad := newPickTestWaitingJob(t, run.ID, "bad", []string{"ubuntu-latest"}, []byte("}{ not a workflow")) // older
		good := newPickTestWaitingJob(t, run.ID, "good", []string{"ubuntu-latest"}, pickTestPayload)           // newer

		runner := &ActionRunner{UUID: "r-parse-skip", Name: "r-parse-skip", RepoID: pickTestRepoID, AgentLabels: []string{"ubuntu-latest"}}
		require.NoError(t, db.Insert(t.Context(), runner))

		task, ok, err := CreateTaskForRunner(t.Context(), runner)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, good.ID, task.JobID)

		failed := unittest.AssertExistsAndLoadBean(t, &ActionRunJob{ID: bad.ID})
		assert.Equal(t, StatusFailure, failed.Status, "unparsable job must be marked failed, not left waiting")
	})

	// #37586: a global runner can select a job whose run was deleted (repo-scoped
	// runners filter those out via the run_id subquery). LoadAttributes then fails
	// with not-exist, which must fail the job instead of stalling every poll forever.
	t.Run("orphaned job (deleted run) is failed for a global runner", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		// isolate the global runner from fixture waiting jobs
		_, err := db.GetEngine(t.Context()).
			Where(builder.In("status", StatusWaiting, StatusBlocked)).
			Cols("status").Update(&ActionRunJob{Status: StatusCancelled})
		require.NoError(t, err)

		const missingRunID = 9_999_999
		orphan := newPickTestWaitingJob(t, missingRunID, "orphan", []string{"ubuntu-latest"}, pickTestPayload)

		runner := &ActionRunner{UUID: "r-global-orphan", Name: "r-global-orphan", AgentLabels: []string{"ubuntu-latest"}}
		require.NoError(t, db.Insert(t.Context(), runner))

		task, ok, err := CreateTaskForRunner(t.Context(), runner)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Nil(t, task)

		failed := unittest.AssertExistsAndLoadBean(t, &ActionRunJob{ID: orphan.ID})
		assert.Equal(t, StatusFailure, failed.Status)
	})
}

func TestTaskCancellingFinalizesToCancelled(t *testing.T) {
	testResult := func(t *testing.T, result runnerv1.Result) {
		t.Helper()
		require.NoError(t, unittest.PrepareTestDatabase())

		task, job := newRunningTaskForCancelling(t, "cancelling-finalization-job", true)
		require.NoError(t, StopTask(t.Context(), task.ID, StatusCancelling))

		taskAfterStop := unittest.AssertExistsAndLoadBean(t, &ActionTask{ID: task.ID})
		assert.Equal(t, StatusCancelling, taskAfterStop.Status)

		updatedTask, err := UpdateTaskByState(t.Context(), task.RunnerID, &runnerv1.TaskState{
			Id:        task.ID,
			Result:    result,
			StoppedAt: timestamppb.Now(),
		})
		require.NoError(t, err)
		assert.Equal(t, StatusCancelled, updatedTask.Status)

		taskAfterUpdate := unittest.AssertExistsAndLoadBean(t, &ActionTask{ID: task.ID})
		assert.Equal(t, StatusCancelled, taskAfterUpdate.Status)

		jobAfterUpdate := unittest.AssertExistsAndLoadBean(t, &ActionRunJob{ID: job.ID})
		assert.Equal(t, StatusCancelled, jobAfterUpdate.Status)
	}

	t.Run("runner reports success", func(t *testing.T) {
		testResult(t, runnerv1.Result_RESULT_SUCCESS)
	})

	t.Run("runner reports failure", func(t *testing.T) {
		testResult(t, runnerv1.Result_RESULT_FAILURE)
	})
}

// TestStopTaskCancellingFallsBackToCancelled covers the cases where the cancelling handshake can
// never complete, so StopTask must cancel right away instead of waiting for the zombie task cleanup.
func TestStopTaskCancellingFallsBackToCancelled(t *testing.T) {
	assertCancelled := func(t *testing.T, task *ActionTask, job *ActionRunJob) {
		t.Helper()

		taskAfterStop := unittest.AssertExistsAndLoadBean(t, &ActionTask{ID: task.ID})
		assert.Equal(t, StatusCancelled, taskAfterStop.Status)
		assert.NotZero(t, taskAfterStop.Stopped)

		jobAfterStop := unittest.AssertExistsAndLoadBean(t, &ActionRunJob{ID: job.ID})
		assert.Equal(t, StatusCancelled, jobAfterStop.Status)
		assert.NotZero(t, jobAfterStop.Stopped)
	}

	// A runner too old to know the cancelling state can only be stopped by a final status.
	t.Run("legacy runner", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		task, job := newRunningTaskForCancelling(t, "legacy-cancelling-job", false)

		require.NoError(t, StopTask(t.Context(), task.ID, StatusCancelling))
		assertCancelled(t, task, job)
	})

	// The runner is gone, e.g. an ephemeral runner was cleaned up, so nobody can acknowledge.
	t.Run("missing runner", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		task, job := newRunningTaskForCancelling(t, "missing-runner-cancelling-job", true)

		_, err := db.DeleteByID[ActionRunner](t.Context(), task.RunnerID)
		require.NoError(t, err)

		require.NoError(t, StopTask(t.Context(), task.ID, StatusCancelling))
		assertCancelled(t, task, job)
	})

	// The runner went silent, e.g. it gave up while Gitea was restarting, so it never picks up the request.
	t.Run("silent runner", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		task, job := newRunningTaskForCancelling(t, "silent-runner-cancelling-job", true)

		// NoAutoTime because the point of the test is an "updated" older than xorm would write
		task.Updated = timeutil.TimeStampNow().AddDuration(-2 * taskReportTimeout)
		_, err := db.GetEngine(t.Context()).ID(task.ID).Cols("updated").NoAutoTime().Update(task)
		require.NoError(t, err)

		require.NoError(t, StopTask(t.Context(), task.ID, StatusCancelling))
		assertCancelled(t, task, job)

		// A runner coming back still learns the outcome from the UpdateTask response,
		// and its late result does not overwrite the cancellation.
		late, err := UpdateTaskByState(t.Context(), task.RunnerID, &runnerv1.TaskState{
			Id:        task.ID,
			Result:    runnerv1.Result_RESULT_SUCCESS,
			StoppedAt: timestamppb.Now(),
		})
		require.NoError(t, err)
		assert.Equal(t, StatusCancelled, late.Status)
		assert.Equal(t, runnerv1.Result_RESULT_CANCELLED, late.Status.AsResult())
	})
}

// TestStopTaskCancellingKeepsReportTime makes sure persisting the cancelling status does not refresh
// "updated": re-cancelling would otherwise defer both the fallback to cancelled and the zombie task
// cleanup, no matter how long the runner has been silent.
func TestStopTaskCancellingKeepsReportTime(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	task, _ := newRunningTaskForCancelling(t, "repeated-cancelling-job", true)

	// NoAutoTime because the point of the test is an "updated" older than xorm would write
	lastReport := timeutil.TimeStampNow().AddDuration(-taskReportTimeout / 2)
	task.Updated = lastReport
	_, err := db.GetEngine(t.Context()).ID(task.ID).Cols("updated").NoAutoTime().Update(task)
	require.NoError(t, err)

	// the runner reported recently enough, so the task waits for it to acknowledge
	require.NoError(t, StopTask(t.Context(), task.ID, StatusCancelling))
	taskAfterStop := unittest.AssertExistsAndLoadBean(t, &ActionTask{ID: task.ID})
	assert.Equal(t, StatusCancelling, taskAfterStop.Status)
	assert.Equal(t, lastReport, taskAfterStop.Updated)

	// cancelling it again does not reset the clock either
	require.NoError(t, StopTask(t.Context(), task.ID, StatusCancelling))
	taskAfterSecondStop := unittest.AssertExistsAndLoadBean(t, &ActionTask{ID: task.ID})
	assert.Equal(t, StatusCancelling, taskAfterSecondStop.Status)
	assert.Equal(t, lastReport, taskAfterSecondStop.Updated)
}

// TestReleaseTaskForRunner verifies that releasing a freshly-claimed task returns
// its job to the waiting queue and deletes the task and its steps, so a failure
// while assembling the runner response cannot strand the job in running state.
func TestReleaseTaskForRunner(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	run := &ActionRun{
		Title:         "release-task-test-run",
		RepoID:        1,
		OwnerID:       2,
		WorkflowID:    "test.yaml",
		Index:         9902,
		TriggerUserID: 2,
		Ref:           "refs/heads/main",
		CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:         "push",
		TriggerEvent:  "push",
		Status:        StatusWaiting,
	}
	require.NoError(t, db.Insert(t.Context(), run))

	job := &ActionRunJob{
		RunID:           run.ID,
		RepoID:          run.RepoID,
		OwnerID:         run.OwnerID,
		CommitSHA:       run.CommitSHA,
		Name:            "release-job",
		Attempt:         1,
		JobID:           "release-job",
		Status:          StatusWaiting,
		RunsOn:          []string{"ubuntu-latest"},
		WorkflowPayload: []byte("on: push\njobs:\n  release-job:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hi\n"),
	}
	require.NoError(t, db.Insert(t.Context(), job))

	runner := &ActionRunner{
		UUID:        "release-runner-uuid",
		Name:        "release-runner",
		AgentLabels: []string{"ubuntu-latest"},
	}
	runner.GenerateAndFillToken()
	require.NoError(t, db.Insert(t.Context(), runner))

	task, ok, err := CreateTaskForRunner(t.Context(), runner)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, task)

	claimed := unittest.AssertExistsAndLoadBean(t, &ActionRunJob{ID: job.ID})
	require.Equal(t, StatusRunning, claimed.Status)
	require.Equal(t, task.ID, claimed.TaskID)

	require.NoError(t, ReleaseTaskForRunner(t.Context(), task))

	// Job is back in the waiting queue with no task assigned.
	released := unittest.AssertExistsAndLoadBean(t, &ActionRunJob{ID: job.ID})
	assert.Equal(t, StatusWaiting, released.Status)
	assert.Zero(t, released.TaskID)
	assert.Zero(t, released.Started)

	// The task and its steps are gone.
	unittest.AssertNotExistsBean(t, &ActionTask{ID: task.ID})
	unittest.AssertNotExistsBean(t, &ActionTaskStep{TaskID: task.ID})
}

// newRunningTaskForCancelling inserts a running run/job/task assigned to a fresh runner,
// which is the state every cancellation test starts from.
func newRunningTaskForCancelling(t *testing.T, name string, hasCancellingSupport bool) (*ActionTask, *ActionRunJob) {
	t.Helper()

	run := &ActionRun{
		Title:         "cancelling-test-run",
		RepoID:        1,
		OwnerID:       2,
		WorkflowID:    "test.yaml",
		Index:         999,
		TriggerUserID: 2,
		Ref:           "refs/heads/master",
		CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:         "push",
		TriggerEvent:  "push",
		Status:        StatusRunning,
		Started:       timeutil.TimeStampNow(),
	}
	require.NoError(t, db.Insert(t.Context(), run))

	job := &ActionRunJob{
		RunID:     run.ID,
		RepoID:    run.RepoID,
		OwnerID:   run.OwnerID,
		CommitSHA: run.CommitSHA,
		Name:      name,
		Attempt:   1,
		JobID:     name,
		Status:    StatusRunning,
	}
	require.NoError(t, db.Insert(t.Context(), job))

	runner := &ActionRunner{
		UUID:                 name,
		Name:                 name,
		HasCancellingSupport: hasCancellingSupport,
	}
	require.NoError(t, db.Insert(t.Context(), runner))

	task := &ActionTask{
		JobID:     job.ID,
		Attempt:   1,
		RunnerID:  runner.ID,
		Status:    StatusRunning,
		Started:   timeutil.TimeStampNow(),
		RepoID:    run.RepoID,
		OwnerID:   run.OwnerID,
		CommitSHA: run.CommitSHA,
	}
	require.NoError(t, db.Insert(t.Context(), task))

	job.TaskID = task.ID
	_, err := UpdateRunJob(t.Context(), job, nil, "task_id")
	require.NoError(t, err)

	return task, job
}
