// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFailedJobsForRerun(t *testing.T) {
	makeJob := func(id int64, jobID string, status actions_model.Status, needs ...string) *actions_model.ActionRunJob {
		return &actions_model.ActionRunJob{ID: id, JobID: jobID, Status: status, Needs: needs}
	}

	t.Run("no failed jobs returns empty", func(t *testing.T) {
		jobs := []*actions_model.ActionRunJob{
			makeJob(1, "job1", actions_model.StatusSuccess),
			makeJob(2, "job2", actions_model.StatusSkipped, "job1"),
		}
		assert.Empty(t, GetFailedJobsForRerun(jobs))
	})

	t.Run("single failed job with no dependents", func(t *testing.T) {
		job1 := makeJob(1, "job1", actions_model.StatusFailure)
		job2 := makeJob(2, "job2", actions_model.StatusSuccess)
		jobs := []*actions_model.ActionRunJob{job1, job2}

		result := GetFailedJobsForRerun(jobs)
		assert.ElementsMatch(t, []*actions_model.ActionRunJob{job1}, result)
	})

	t.Run("failed job does not pull in downstream dependents", func(t *testing.T) {
		job1 := makeJob(1, "job1", actions_model.StatusFailure)
		job2 := makeJob(2, "job2", actions_model.StatusSkipped, "job1")
		job3 := makeJob(3, "job3", actions_model.StatusSkipped, "job2")
		job4 := makeJob(4, "job4", actions_model.StatusSuccess) // unrelated, must not appear
		jobs := []*actions_model.ActionRunJob{job1, job2, job3, job4}

		result := GetFailedJobsForRerun(jobs)
		assert.ElementsMatch(t, []*actions_model.ActionRunJob{job1}, result)
	})

	t.Run("multiple failed jobs are returned directly", func(t *testing.T) {
		job1 := makeJob(1, "job1", actions_model.StatusFailure)
		job2 := makeJob(2, "job2", actions_model.StatusFailure)
		job3 := makeJob(3, "job3", actions_model.StatusSkipped, "job1")
		job4 := makeJob(4, "job4", actions_model.StatusSkipped, "job2")
		jobs := []*actions_model.ActionRunJob{job1, job2, job3, job4}

		result := GetFailedJobsForRerun(jobs)
		assert.ElementsMatch(t, []*actions_model.ActionRunJob{job1, job2}, result)
	})

	t.Run("shared downstream dependent is not included", func(t *testing.T) {
		job1 := makeJob(1, "job1", actions_model.StatusFailure)
		job2 := makeJob(2, "job2", actions_model.StatusFailure)
		job3 := makeJob(3, "job3", actions_model.StatusSkipped, "job1", "job2")
		jobs := []*actions_model.ActionRunJob{job1, job2, job3}

		result := GetFailedJobsForRerun(jobs)
		assert.ElementsMatch(t, []*actions_model.ActionRunJob{job1, job2}, result)
		assert.Len(t, result, 2)
	})

	t.Run("successful downstream job of a failed job is not included", func(t *testing.T) {
		job1 := makeJob(1, "job1", actions_model.StatusFailure)
		job2 := makeJob(2, "job2", actions_model.StatusSuccess, "job1")
		jobs := []*actions_model.ActionRunJob{job1, job2}

		result := GetFailedJobsForRerun(jobs)
		assert.ElementsMatch(t, []*actions_model.ActionRunJob{job1}, result)
	})
}

func TestRerunValidation(t *testing.T) {
	runningRun := &actions_model.ActionRun{Status: actions_model.StatusRunning}

	t.Run("RerunWorkflowRunJobs rejects a non-done run", func(t *testing.T) {
		jobs := []*actions_model.ActionRunJob{
			{ID: 1, JobID: "job1"},
		}
		_, err := RerunWorkflowRunJobs(t.Context(), nil, runningRun, &user_model.User{ID: 1}, jobs)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})

	t.Run("RerunWorkflowRunJobs rejects a non-done run when failed jobs exist", func(t *testing.T) {
		jobs := []*actions_model.ActionRunJob{
			{ID: 1, JobID: "job1", Status: actions_model.StatusFailure},
		}
		_, err := RerunWorkflowRunJobs(t.Context(), nil, runningRun, &user_model.User{ID: 1}, GetFailedJobsForRerun(jobs))
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})
}
