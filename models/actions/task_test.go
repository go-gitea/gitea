// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strings"
	"testing"

	runnerv1 "gitea.dev/actions-proto-go/runner/v1"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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
	newRunningTask := func(t *testing.T) (*ActionTask, *ActionRunJob) {
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
			Name:      "cancelling-finalization-job",
			Attempt:   1,
			JobID:     "cancelling-finalization-job",
			Status:    StatusRunning,
		}
		require.NoError(t, db.Insert(t.Context(), job))

		runner := &ActionRunner{
			UUID:                 "runner-cancelling-supported",
			Name:                 "runner-cancelling-supported",
			HasCancellingSupport: true,
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

	testResult := func(t *testing.T, result runnerv1.Result) {
		t.Helper()
		require.NoError(t, unittest.PrepareTestDatabase())

		task, job := newRunningTask(t)
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

func TestStopTaskCancellingFallsBackForLegacyRunner(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

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
		Name:      "legacy-cancelling-job",
		Attempt:   1,
		JobID:     "legacy-cancelling-job",
		Status:    StatusRunning,
	}
	require.NoError(t, db.Insert(t.Context(), job))

	runner := &ActionRunner{
		UUID:                 "runner-legacy-no-cancelling",
		Name:                 "runner-legacy-no-cancelling",
		HasCancellingSupport: false,
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

	require.NoError(t, StopTask(t.Context(), task.ID, StatusCancelling))

	taskAfterStop := unittest.AssertExistsAndLoadBean(t, &ActionTask{ID: task.ID})
	assert.Equal(t, StatusCancelled, taskAfterStop.Status)
	assert.NotZero(t, taskAfterStop.Stopped)

	jobAfterStop := unittest.AssertExistsAndLoadBean(t, &ActionRunJob{ID: job.ID})
	assert.Equal(t, StatusCancelled, jobAfterStop.Status)
	assert.NotZero(t, jobAfterStop.Stopped)
}

func TestStopTaskCancellingFallsBackForMissingRunner(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

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
		Name:      "missing-runner-cancelling-job",
		Attempt:   1,
		JobID:     "missing-runner-cancelling-job",
		Status:    StatusRunning,
	}
	require.NoError(t, db.Insert(t.Context(), job))

	runner := &ActionRunner{
		UUID:                 "runner-cleaned-up-before-cancel",
		Name:                 "runner-cleaned-up-before-cancel",
		HasCancellingSupport: true,
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

	_, err = db.DeleteByID[ActionRunner](t.Context(), runner.ID)
	require.NoError(t, err)

	require.NoError(t, StopTask(t.Context(), task.ID, StatusCancelling))

	taskAfterStop := unittest.AssertExistsAndLoadBean(t, &ActionTask{ID: task.ID})
	assert.Equal(t, StatusCancelled, taskAfterStop.Status)
	assert.NotZero(t, taskAfterStop.Stopped)

	jobAfterStop := unittest.AssertExistsAndLoadBean(t, &ActionRunJob{ID: job.ID})
	assert.Equal(t, StatusCancelled, jobAfterStop.Status)
	assert.NotZero(t, jobAfterStop.Stopped)
}
