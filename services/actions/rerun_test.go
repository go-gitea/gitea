// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/util"

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

func TestCloneRunJobForAttempt(t *testing.T) {
	attempt := &actions_model.ActionRunAttempt{ID: 42, Attempt: 2}

	t.Run("preserves continue-on-error", func(t *testing.T) {
		template := &actions_model.ActionRunJob{ContinueOnError: true, Status: actions_model.StatusFailure}
		clone := cloneRunJobForAttempt(template, attempt)
		assert.True(t, clone.ContinueOnError)
	})

	t.Run("defaults to false when template has it unset", func(t *testing.T) {
		template := &actions_model.ActionRunJob{ContinueOnError: false}
		clone := cloneRunJobForAttempt(template, attempt)
		assert.False(t, clone.ContinueOnError)
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

func TestRerunPlan(t *testing.T) {
	// "verify" appears in two scopes (inner caller under deploy, and top-level) so scope-blind matching would fail here.

	//	build              id=101, attemptJobID=1
	//	test               id=102, attemptJobID=2,  needs=[build]
	//	deploy             id=103, attemptJobID=3,  caller
	//	├── validate       id=104, attemptJobID=4,  parent=103
	//	├── push           id=105, attemptJobID=5,  parent=103, needs=[validate]
	//	├── verify         id=106, attemptJobID=6,  parent=103, caller, needs=[push]
	//	│   ├── smoke-test id=107, attemptJobID=7,  parent=106
	//	│   └── cleanup    id=108, attemptJobID=8,  parent=106, needs=[smoke-test]
	//	└── finish-deploy  id=109, attemptJobID=9,  parent=103, needs=[verify]
	//	verify             id=110, attemptJobID=10, needs=[deploy] (top-level, same JobID)

	buildJob := templateJob(101, 1, "build", 0, false)
	testJob := templateJob(102, 2, "test", 0, false, "build")
	deployJob := templateJob(103, 3, "deploy", 0, true)
	validateJob := templateJob(104, 4, "validate", 103, false)
	pushJob := templateJob(105, 5, "push", 103, false, "validate")
	verifyInnerJob := templateJob(106, 6, "verify", 103, true, "push")
	smokeTestJob := templateJob(107, 7, "smoke-test", 106, false)
	cleanupJob := templateJob(108, 8, "cleanup", 106, false, "smoke-test")
	finishDeployJob := templateJob(109, 9, "finish-deploy", 103, false, "verify")
	verifyTopJob := templateJob(110, 10, "verify", 0, false, "deploy")

	jobs := []*actions_model.ActionRunJob{
		buildJob, testJob, deployJob, validateJob, pushJob,
		verifyInnerJob, smokeTestJob, cleanupJob,
		finishDeployJob, verifyTopJob,
	}

	t.Run("ExpandRerunJobIDs", func(t *testing.T) {
		t.Run("empty jobsToRerun reruns every template job, no ancestors", func(t *testing.T) {
			plan := &rerunPlan{templateJobs: jobs}
			require.NoError(t, plan.expandRerunJobIDs(nil))

			assert.ElementsMatch(t, attemptJobIDsOf(jobs...), plan.rerunAttemptJobIDs.Values())
			assert.Empty(t, plan.ancestorAttemptJobIDs)
		})

		t.Run("same-scope downstream BFS pulls in dependents", func(t *testing.T) {
			// a -> b -> c (chain), d unrelated.
			a := templateJob(101, 1, "a", 0, false)
			b := templateJob(102, 2, "b", 0, false, "a")
			c := templateJob(103, 3, "c", 0, false, "b")
			d := templateJob(104, 4, "d", 0, false)
			plan := &rerunPlan{templateJobs: []*actions_model.ActionRunJob{a, b, c, d}}
			require.NoError(t, plan.expandRerunJobIDs([]*actions_model.ActionRunJob{a}))

			assert.ElementsMatch(t, attemptJobIDsOf(a, b, c), plan.rerunAttemptJobIDs.Values())
			assert.Empty(t, plan.ancestorAttemptJobIDs)
		})

		t.Run("rerun a deep child escalates across reusable scopes", func(t *testing.T) {
			plan := &rerunPlan{templateJobs: jobs}
			require.NoError(t, plan.expandRerunJobIDs([]*actions_model.ActionRunJob{smokeTestJob}))

			// rerun: smoke-test (selected), cleanup (same-scope downstream),
			// finish-deploy (deploy-scope sibling of inner verify ancestor),
			// top-level verify (top-scope sibling of deploy ancestor).
			assert.ElementsMatch(t,
				attemptJobIDsOf(smokeTestJob, cleanupJob, finishDeployJob, verifyTopJob),
				plan.rerunAttemptJobIDs.Values())

			// ancestors: inner verify and deploy
			assert.ElementsMatch(t, attemptJobIDsOf(verifyInnerJob, deployJob), plan.ancestorAttemptJobIDs.Values())
		})

		t.Run("rerun a top-level caller resets only itself and same-scope dependents", func(t *testing.T) {
			plan := &rerunPlan{templateJobs: jobs}
			require.NoError(t, plan.expandRerunJobIDs([]*actions_model.ActionRunJob{deployJob}))

			// rerun: deploy (selected) + top-level verify (needs:[deploy]).
			assert.ElementsMatch(t, attemptJobIDsOf(deployJob, verifyTopJob), plan.rerunAttemptJobIDs.Values())
			// deploy is top-level so no ancestors.
			assert.Empty(t, plan.ancestorAttemptJobIDs)
		})

		t.Run("rerun a nested caller escalates one level", func(t *testing.T) {
			plan := &rerunPlan{templateJobs: jobs}
			require.NoError(t, plan.expandRerunJobIDs([]*actions_model.ActionRunJob{verifyInnerJob}))

			// inner verify (selected) -> finish-deploy (deploy-scope dep) -> top-level verify (top-scope dep of deploy).
			assert.ElementsMatch(t,
				attemptJobIDsOf(verifyInnerJob, finishDeployJob, verifyTopJob),
				plan.rerunAttemptJobIDs.Values())
			// deploy is the only ancestor (one level up from inner verify).
			assert.ElementsMatch(t, attemptJobIDsOf(deployJob), plan.ancestorAttemptJobIDs.Values())
		})

		t.Run("selecting one same-name job leaves the other-scope same-name job alone", func(t *testing.T) {
			// Selecting the top-level "verify" must not pull in the same-named inner one or its descendants.
			plan := &rerunPlan{templateJobs: jobs}
			require.NoError(t, plan.expandRerunJobIDs([]*actions_model.ActionRunJob{verifyTopJob}))

			// Only the top-level verify is rerun.
			assert.ElementsMatch(t, attemptJobIDsOf(verifyTopJob), plan.rerunAttemptJobIDs.Values())
			assert.Empty(t, plan.ancestorAttemptJobIDs)
		})

		t.Run("a caller is rerun when a sibling it needs is selected", func(t *testing.T) {
			plan := &rerunPlan{templateJobs: jobs}
			require.NoError(t, plan.expandRerunJobIDs([]*actions_model.ActionRunJob{pushJob}))

			assert.ElementsMatch(t,
				attemptJobIDsOf(pushJob, verifyInnerJob, finishDeployJob, verifyTopJob),
				plan.rerunAttemptJobIDs.Values())
			assert.ElementsMatch(t, attemptJobIDsOf(deployJob), plan.ancestorAttemptJobIDs.Values())

			// Confirm the downstream effect: verify(inner) is a reset caller, so its children's DB row IDs are marked for skip-clone.
			assert.ElementsMatch(t, rowIDsOf(smokeTestJob, cleanupJob), plan.collectResetCallerDescendants().Values())
		})

		t.Run("multiple selections converge", func(t *testing.T) {
			plan := &rerunPlan{templateJobs: jobs}
			require.NoError(t, plan.expandRerunJobIDs([]*actions_model.ActionRunJob{deployJob, smokeTestJob}))

			assert.ElementsMatch(t, attemptJobIDsOf(deployJob, smokeTestJob, cleanupJob, finishDeployJob, verifyTopJob), plan.rerunAttemptJobIDs.Values())
			assert.Empty(t, plan.ancestorAttemptJobIDs)
			assert.ElementsMatch(t,
				rowIDsOf(validateJob, pushJob, verifyInnerJob, smokeTestJob, cleanupJob, finishDeployJob),
				plan.collectResetCallerDescendants().Values())
		})
	})

	t.Run("CollectResetCallerDescendants", func(t *testing.T) {
		planWith := func(rerunJobs ...*actions_model.ActionRunJob) *rerunPlan {
			set := make(container.Set[int64])
			for _, j := range rerunJobs {
				set.Add(j.AttemptJobID)
			}
			return &rerunPlan{templateJobs: jobs, rerunAttemptJobIDs: set}
		}

		t.Run("non-caller in reset set is ignored", func(t *testing.T) {
			assert.Empty(t, planWith(smokeTestJob).collectResetCallerDescendants())
		})

		t.Run("caller in reset set returns transitive descendants", func(t *testing.T) {
			out := planWith(deployJob).collectResetCallerDescendants()
			assert.ElementsMatch(t,
				rowIDsOf(validateJob, pushJob, verifyInnerJob, smokeTestJob, cleanupJob, finishDeployJob),
				out.Values())
		})

		t.Run("multiple reset callers union their descendants", func(t *testing.T) {
			out := planWith(deployJob, verifyInnerJob).collectResetCallerDescendants()
			assert.ElementsMatch(t,
				rowIDsOf(validateJob, pushJob, verifyInnerJob, smokeTestJob, cleanupJob, finishDeployJob),
				out.Values())
		})

		t.Run("nested-only reset returns just the nested subtree", func(t *testing.T) {
			out := planWith(verifyInnerJob).collectResetCallerDescendants()
			assert.ElementsMatch(t, rowIDsOf(smokeTestJob, cleanupJob), out.Values())
		})
	})

	t.Run("HasRerunDependency", func(t *testing.T) {
		t.Run("no needs returns false", func(t *testing.T) {
			plan := &rerunPlan{
				templateJobs:          []*actions_model.ActionRunJob{buildJob},
				rerunAttemptJobIDs:    make(container.Set[int64]),
				ancestorAttemptJobIDs: make(container.Set[int64]),
			}
			assert.False(t, plan.hasRerunDependency(buildJob))
		})

		t.Run("dependency in rerun set returns true", func(t *testing.T) {
			plan := &rerunPlan{
				templateJobs:          jobs,
				rerunAttemptJobIDs:    container.SetOf(smokeTestJob.AttemptJobID),
				ancestorAttemptJobIDs: make(container.Set[int64]),
			}
			// cleanup `needs: [smoke-test]`, both in inner verify scope.
			assert.True(t, plan.hasRerunDependency(cleanupJob))
		})

		t.Run("dependency in ancestor set returns true", func(t *testing.T) {
			plan := &rerunPlan{
				templateJobs:          jobs,
				rerunAttemptJobIDs:    container.SetOf(attemptJobIDsOf(smokeTestJob, cleanupJob)...),
				ancestorAttemptJobIDs: container.SetOf(verifyInnerJob.AttemptJobID),
			}
			assert.True(t, plan.hasRerunDependency(finishDeployJob))
		})

		t.Run("dependency on unrelated sibling returns false", func(t *testing.T) {
			plan := &rerunPlan{
				templateJobs:          jobs,
				rerunAttemptJobIDs:    container.SetOf(smokeTestJob.AttemptJobID),
				ancestorAttemptJobIDs: make(container.Set[int64]),
			}
			assert.False(t, plan.hasRerunDependency(pushJob))
		})

		t.Run("scope-bound: same JobID in another scope does not match", func(t *testing.T) {
			plan := &rerunPlan{
				templateJobs:          jobs,
				rerunAttemptJobIDs:    container.SetOf(verifyTopJob.AttemptJobID),
				ancestorAttemptJobIDs: make(container.Set[int64]),
			}
			assert.False(t, plan.hasRerunDependency(finishDeployJob))

			// Sanity: swap to the inner verify and the same target now sees it.
			plan.rerunAttemptJobIDs = container.SetOf(verifyInnerJob.AttemptJobID)
			assert.True(t, plan.hasRerunDependency(finishDeployJob))
		})
	})
}

// templateJob is a small constructor for fixture jobs used by the rerunPlan unit tests.
func templateJob(id, attemptJobID int64, jobID string, parentID int64, isCaller bool, needs ...string) *actions_model.ActionRunJob {
	return &actions_model.ActionRunJob{
		ID:               id,
		AttemptJobID:     attemptJobID,
		JobID:            jobID,
		ParentJobID:      parentID,
		IsReusableCaller: isCaller,
		Needs:            needs,
	}
}

func attemptJobIDsOf(jobs ...*actions_model.ActionRunJob) []int64 {
	out := make([]int64, len(jobs))
	for i, j := range jobs {
		out[i] = j.AttemptJobID
	}
	return out
}

func rowIDsOf(jobs ...*actions_model.ActionRunJob) []int64 {
	out := make([]int64, len(jobs))
	for i, j := range jobs {
		out[i] = j.ID
	}
	return out
}
