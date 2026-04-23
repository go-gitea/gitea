// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/actions/jobparser"
	"code.gitea.io/gitea/modules/timeutil"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
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
	ensureUserExists := func(t *testing.T, id int64, name string) {
		t.Helper()

		exists, err := db.GetEngine(t.Context()).ID(id).Exist(&user_model.User{})
		require.NoError(t, err)
		if exists {
			return
		}

		u := &user_model.User{
			ID:          id,
			LowerName:   strings.ToLower(name),
			Name:        name,
			Email:       name + "@example.com",
			Passwd:      "not-used",
			Avatar:      "",
			AvatarEmail: name + "@example.com",
			IsActive:    true,
		}
		require.NoError(t, db.Insert(t.Context(), u))
	}

	newRunningTask := func(t *testing.T) (*ActionTask, *ActionRunJob) {
		t.Helper()

		run := unittest.AssertExistsAndLoadBean(t, &ActionRun{ID: 793})

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

		task := &ActionTask{
			JobID:     job.ID,
			Attempt:   1,
			RunnerID:  999999,
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

		ensureUserExists(t, 1, "user1")
		ensureUserExists(t, 5, "user5")

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
