// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	runnerv1 "gitea.dev/actionslib/runner/v1"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	"xorm.io/xorm/contexts"
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

// TestCreateTaskForRunnerPagination verifies that a job sitting beyond the first page is still claimed
func TestCreateTaskForRunnerPagination(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	defer func(orig int) { pickTaskBatchSize = orig }(pickTaskBatchSize)
	pickTaskBatchSize = 2

	run := &ActionRun{
		Title:         "pagination-test-run",
		RepoID:        1,
		OwnerID:       2,
		WorkflowID:    "test.yaml",
		Index:         9903,
		TriggerUserID: 2,
		Ref:           "refs/heads/main",
		CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:         "push",
		TriggerEvent:  "push",
		Status:        StatusWaiting,
	}
	require.NoError(t, db.Insert(t.Context(), run))

	// Five waiting jobs the runner cannot run, then one it can.
	// With a page size of 2 the matching job only appears on the third page.
	for i := range 5 {
		mismatch := &ActionRunJob{
			RunID:     run.ID,
			RepoID:    run.RepoID,
			OwnerID:   run.OwnerID,
			CommitSHA: run.CommitSHA,
			Name:      "mismatch-" + string(rune('a'+i)),
			Attempt:   1,
			JobID:     "mismatch-" + string(rune('a'+i)),
			Status:    StatusWaiting,
			RunsOn:    []string{"windows-latest"},
		}
		require.NoError(t, db.Insert(t.Context(), mismatch))
	}

	target := &ActionRunJob{
		RunID:           run.ID,
		RepoID:          run.RepoID,
		OwnerID:         run.OwnerID,
		CommitSHA:       run.CommitSHA,
		Name:            "target-job",
		Attempt:         1,
		JobID:           "target-job",
		Status:          StatusWaiting,
		RunsOn:          []string{"ubuntu-latest"},
		WorkflowPayload: []byte("on: push\njobs:\n  target-job:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hi\n"),
	}
	require.NoError(t, db.Insert(t.Context(), target))

	runner := &ActionRunner{
		UUID:        "pagination-runner-uuid",
		Name:        "pagination-runner",
		AgentLabels: []string{"ubuntu-latest"},
	}
	runner.GenerateAndFillToken()
	require.NoError(t, db.Insert(t.Context(), runner))

	task, ok, err := CreateTaskForRunner(t.Context(), runner)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, task)

	claimed := unittest.AssertExistsAndLoadBean(t, &ActionRunJob{ID: target.ID})
	assert.Equal(t, StatusRunning, claimed.Status)
	assert.Equal(t, task.ID, claimed.TaskID)
}

type failFirstStepWrite struct{ fired atomic.Bool }

func (h *failFirstStepWrite) BeforeProcess(c *contexts.ContextHook) (context.Context, error) {
	if !h.fired.Load() && strings.HasPrefix(c.SQL, "UPDATE") && strings.Contains(c.SQL, "action_task_step") {
		h.fired.Store(true)
		return nil, errors.New("interrupted")
	}
	return c.Ctx, nil
}

func (*failFirstStepWrite) AfterProcess(*contexts.ContextHook) error { return nil }

// TestUpdateTaskByStateIsAtomic checks that an interrupted report writes nothing: a surviving task or
// job write would hit the "state is final" early return, which no retry or cleanup repairs.
func TestUpdateTaskByStateIsAtomic(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	task, job := newRunningTaskForCancelling(t, "atomic-report-job", true)
	require.NoError(t, db.Insert(t.Context(), &ActionTaskStep{TaskID: task.ID, RepoID: task.RepoID, Status: StatusRunning}))
	unittest.GetXORMEngine().AddHook(&failFirstStepWrite{})
	finalState := &runnerv1.TaskState{Id: task.ID, Result: runnerv1.Result_RESULT_SUCCESS, StoppedAt: timestamppb.Now()}
	_, err := UpdateTaskByState(t.Context(), task.RunnerID, finalState)
	require.Error(t, err)
	assert.Equal(t, StatusRunning, unittest.AssertExistsAndLoadBean(t, &ActionTask{ID: task.ID}).Status)
	assert.Equal(t, StatusRunning, unittest.AssertExistsAndLoadBean(t, &ActionRunJob{ID: job.ID}).Status)
	_, err = UpdateTaskByState(t.Context(), task.RunnerID, finalState)
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, unittest.AssertExistsAndLoadBean(t, &ActionRunJob{ID: job.ID}).Status)
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
