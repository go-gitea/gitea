// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAllRerunJobs(t *testing.T) {
	job1 := &actions_model.ActionRunJob{JobID: "job1"}
	job2 := &actions_model.ActionRunJob{JobID: "job2", Needs: []string{"job1"}}
	job3 := &actions_model.ActionRunJob{JobID: "job3", Needs: []string{"job2"}}
	job4 := &actions_model.ActionRunJob{JobID: "job4", Needs: []string{"job2", "job3"}}

	jobs := []*actions_model.ActionRunJob{job1, job2, job3, job4}

	testCases := []struct {
		job       *actions_model.ActionRunJob
		rerunJobs []*actions_model.ActionRunJob
	}{
		{
			job1,
			[]*actions_model.ActionRunJob{job1, job2, job3, job4},
		},
		{
			job2,
			[]*actions_model.ActionRunJob{job2, job3, job4},
		},
		{
			job3,
			[]*actions_model.ActionRunJob{job3, job4},
		},
		{
			job4,
			[]*actions_model.ActionRunJob{job4},
		},
	}

	for _, tc := range testCases {
		rerunJobs := GetAllRerunJobs(tc.job, jobs)
		assert.ElementsMatch(t, tc.rerunJobs, rerunJobs)
	}
}

func TestGetFailedRerunJobs(t *testing.T) {
	// IDs must be non-zero to distinguish jobs in the dedup set.
	makeJob := func(id int64, jobID string, status actions_model.Status, needs ...string) *actions_model.ActionRunJob {
		return &actions_model.ActionRunJob{ID: id, JobID: jobID, Status: status, Needs: needs}
	}

	t.Run("no failed jobs returns empty", func(t *testing.T) {
		jobs := []*actions_model.ActionRunJob{
			makeJob(1, "job1", actions_model.StatusSuccess),
			makeJob(2, "job2", actions_model.StatusSkipped, "job1"),
		}
		assert.Empty(t, GetFailedRerunJobs(jobs))
	})

	t.Run("single failed job with no dependents", func(t *testing.T) {
		job1 := makeJob(1, "job1", actions_model.StatusFailure)
		job2 := makeJob(2, "job2", actions_model.StatusSuccess)
		jobs := []*actions_model.ActionRunJob{job1, job2}

		result := GetFailedRerunJobs(jobs)
		assert.ElementsMatch(t, []*actions_model.ActionRunJob{job1}, result)
	})

	t.Run("failed job pulls in downstream dependents", func(t *testing.T) {
		// job1 failed; job2 depends on job1 (skipped); job3 depends on job2 (skipped)
		job1 := makeJob(1, "job1", actions_model.StatusFailure)
		job2 := makeJob(2, "job2", actions_model.StatusSkipped, "job1")
		job3 := makeJob(3, "job3", actions_model.StatusSkipped, "job2")
		job4 := makeJob(4, "job4", actions_model.StatusSuccess) // unrelated, must not appear
		jobs := []*actions_model.ActionRunJob{job1, job2, job3, job4}

		result := GetFailedRerunJobs(jobs)
		assert.ElementsMatch(t, []*actions_model.ActionRunJob{job1, job2, job3}, result)
	})

	t.Run("multiple independent failed jobs each pull in their own dependents", func(t *testing.T) {
		// job1 failed -> job3 depends on job1
		// job2 failed -> job4 depends on job2
		job1 := makeJob(1, "job1", actions_model.StatusFailure)
		job2 := makeJob(2, "job2", actions_model.StatusFailure)
		job3 := makeJob(3, "job3", actions_model.StatusSkipped, "job1")
		job4 := makeJob(4, "job4", actions_model.StatusSkipped, "job2")
		jobs := []*actions_model.ActionRunJob{job1, job2, job3, job4}

		result := GetFailedRerunJobs(jobs)
		assert.ElementsMatch(t, []*actions_model.ActionRunJob{job1, job2, job3, job4}, result)
	})

	t.Run("shared downstream dependent is not duplicated", func(t *testing.T) {
		// job1 and job2 both failed; job3 depends on both
		job1 := makeJob(1, "job1", actions_model.StatusFailure)
		job2 := makeJob(2, "job2", actions_model.StatusFailure)
		job3 := makeJob(3, "job3", actions_model.StatusSkipped, "job1", "job2")
		jobs := []*actions_model.ActionRunJob{job1, job2, job3}

		result := GetFailedRerunJobs(jobs)
		assert.ElementsMatch(t, []*actions_model.ActionRunJob{job1, job2, job3}, result)
		assert.Len(t, result, 3) // job3 must appear exactly once
	})

	t.Run("successful downstream job of a failed job is still included", func(t *testing.T) {
		// job1 failed; job2 succeeded but depends on job1 — downstream is always rerun
		// regardless of its own status (GetAllRerunJobs includes all transitive dependents)
		job1 := makeJob(1, "job1", actions_model.StatusFailure)
		job2 := makeJob(2, "job2", actions_model.StatusSuccess, "job1")
		jobs := []*actions_model.ActionRunJob{job1, job2}

		result := GetFailedRerunJobs(jobs)
		assert.ElementsMatch(t, []*actions_model.ActionRunJob{job1, job2}, result)
	})
}

func TestRerunValidation(t *testing.T) {
	runningRun := &actions_model.ActionRun{Status: actions_model.StatusRunning}

	t.Run("RerunWorkflowRunJobs rejects a non-done run", func(t *testing.T) {
		jobs := []*actions_model.ActionRunJob{
			{ID: 1, JobID: "job1"},
		}
		err := RerunWorkflowRunJobs(context.Background(), nil, runningRun, jobs)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})

	t.Run("RerunWorkflowRunJobs rejects a non-done run when failed jobs exist", func(t *testing.T) {
		jobs := []*actions_model.ActionRunJob{
			{ID: 1, JobID: "job1", Status: actions_model.StatusFailure},
		}
		err := RerunWorkflowRunJobs(context.Background(), nil, runningRun, GetFailedRerunJobs(jobs))
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})
}
