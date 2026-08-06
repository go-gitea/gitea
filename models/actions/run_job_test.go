// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPriorAttemptChildrenByParent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// 3 attempts of one run:
	//   1: caller expanded with 3 matrix instances of "work" + non-matrix sibling "summary".
	//   2: caller skipped, no children rows.
	//   3: placeholder "current" attempt for the walkback subtest.

	run := &ActionRun{
		Title:         "prior-children-test",
		RepoID:        4,
		Index:         9501,
		OwnerID:       1,
		WorkflowID:    "matrix.yaml",
		TriggerUserID: 1,
		Ref:           "refs/heads/master",
		CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:         "push",
		TriggerEvent:  "push",
		EventPayload:  "{}",
		Status:        StatusSuccess,
	}
	require.NoError(t, db.Insert(ctx, run))

	const callerAttemptJobID int64 = 9001
	insertAttempt := func(t *testing.T, num int64, status Status) *ActionRunAttempt {
		t.Helper()
		a := &ActionRunAttempt{
			RepoID:        run.RepoID,
			RunID:         run.ID,
			Attempt:       num,
			TriggerUserID: 1,
			Status:        status,
		}
		require.NoError(t, db.Insert(ctx, a))
		return a
	}
	insertCaller := func(t *testing.T, attemptID int64, status Status, expanded bool) *ActionRunJob {
		t.Helper()
		caller := &ActionRunJob{
			RunID:            run.ID,
			RunAttemptID:     attemptID,
			RepoID:           run.RepoID,
			OwnerID:          run.OwnerID,
			CommitSHA:        run.CommitSHA,
			Name:             "caller",
			JobID:            "caller",
			Attempt:          1,
			Status:           status,
			AttemptJobID:     callerAttemptJobID,
			IsReusableCaller: true,
			IsExpanded:       expanded,
		}
		require.NoError(t, db.Insert(ctx, caller))
		return caller
	}
	insertChild := func(t *testing.T, attemptID, parentID, attemptJobID int64, name, jobID string) {
		t.Helper()
		require.NoError(t, db.Insert(ctx, &ActionRunJob{
			RunID:        run.ID,
			RunAttemptID: attemptID,
			RepoID:       run.RepoID,
			OwnerID:      run.OwnerID,
			CommitSHA:    run.CommitSHA,
			Name:         name,
			JobID:        jobID,
			Attempt:      1,
			Status:       StatusSuccess,
			AttemptJobID: attemptJobID,
			ParentJobID:  parentID,
		}))
	}

	attempt1 := insertAttempt(t, 1, StatusSuccess)
	caller1 := insertCaller(t, attempt1.ID, StatusSuccess, true)
	insertChild(t, attempt1.ID, caller1.ID, 101, "work (alpha)", "work")
	insertChild(t, attempt1.ID, caller1.ID, 102, "work (beta)", "work")
	insertChild(t, attempt1.ID, caller1.ID, 103, "work (gamma)", "work")
	insertChild(t, attempt1.ID, caller1.ID, 104, "summary", "summary")

	attempt2 := insertAttempt(t, 2, StatusSkipped)
	insertCaller(t, attempt2.ID, StatusSkipped, false) // no children intentionally

	// both subtests expect attempt 1's expansion, differing only in the "current" attempt id
	assertAttempt1Children := func(t *testing.T, out map[string]map[string]*ActionRunJob) {
		t.Helper()
		// outer map keyed by JobID: "work" has 3 matrix instances, "summary" 1
		assert.Len(t, out, 2)
		assert.Len(t, out["work"], 3, "matrix instances must each get their own inner-map entry")
		assert.Len(t, out["summary"], 1)

		require.NotNil(t, out["work"]["work (alpha)"])
		require.NotNil(t, out["work"]["work (beta)"])
		require.NotNil(t, out["work"]["work (gamma)"])
		require.NotNil(t, out["summary"]["summary"])

		assert.Equal(t, int64(101), out["work"]["work (alpha)"].AttemptJobID)
		assert.Equal(t, int64(102), out["work"]["work (beta)"].AttemptJobID)
		assert.Equal(t, int64(103), out["work"]["work (gamma)"].AttemptJobID)
		assert.Equal(t, int64(104), out["summary"]["summary"].AttemptJobID)
	}

	t.Run("matrix instances and non-matrix sibling are indexed by (JobID, Name)", func(t *testing.T) {
		// "current" = attempt 2; prior = attempt 1, which is the immediately preceding attempt.
		out, err := GetPriorAttemptChildrenByParent(ctx, run.ID, attempt2.ID, callerAttemptJobID)
		require.NoError(t, err)
		assertAttempt1Children(t, out)
	})

	t.Run("walkback past an attempt where the caller had no children", func(t *testing.T) {
		attempt3 := insertAttempt(t, 3, StatusRunning)
		// "current" = attempt 3; the immediately preceding attempt 2 has no children, so the lookup must walk further back to attempt 1.
		out, err := GetPriorAttemptChildrenByParent(ctx, run.ID, attempt3.ID, callerAttemptJobID)
		require.NoError(t, err)
		assertAttempt1Children(t, out)
	})
}

// A reusable caller subtree with a Blocked descendant (e.g. a nested caller stuck on an invalid `uses:`) must aggregate to Cancelled, when the run is cancelled.
func TestCancelJobs_NestedBlockedReusableCaller(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	run := &ActionRun{
		Title:         "cancel-nested-caller",
		RepoID:        4,
		Index:         9701,
		OwnerID:       1,
		WorkflowID:    "caller.yaml",
		TriggerUserID: 1,
		Ref:           "refs/heads/master",
		CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:         "push",
		TriggerEvent:  "push",
		EventPayload:  "{}",
		Status:        StatusBlocked,
	}
	require.NoError(t, db.Insert(ctx, run))

	attempt := &ActionRunAttempt{RepoID: run.RepoID, RunID: run.ID, Attempt: 1, TriggerUserID: 1, Status: StatusBlocked}
	require.NoError(t, db.Insert(ctx, attempt))
	run.LatestAttemptID = attempt.ID
	require.NoError(t, UpdateRun(ctx, run, "latest_attempt_id"))

	newJob := func(name string, attemptJobID, parentID int64, callUses string) *ActionRunJob {
		job := &ActionRunJob{
			RunID:            run.ID,
			RunAttemptID:     attempt.ID,
			RepoID:           run.RepoID,
			OwnerID:          run.OwnerID,
			CommitSHA:        run.CommitSHA,
			Name:             name,
			JobID:            name,
			Attempt:          1,
			Status:           StatusBlocked,
			AttemptJobID:     attemptJobID,
			IsReusableCaller: true,
			CallUses:         callUses,
			ParentJobID:      parentID,
		}
		require.NoError(t, db.Insert(ctx, job))
		return job
	}

	// outer: a valid top-level caller that expanded; inner: a nested caller stuck Blocked (invalid uses, never expands).
	outer := newJob("outer", 1, 0, "./.gitea/workflows/lib.yml")
	inner := newJob("inner", 2, outer.ID, "https://other.example.com/o/r/.gitea/workflows/ci.yml@v1")

	// Cancel all jobs of the attempt, ordered by id (parent before child).
	jobs, err := GetRunJobsByRunAndAttemptID(ctx, run.ID, attempt.ID)
	require.NoError(t, err)
	_, err = CancelJobs(ctx, jobs)
	require.NoError(t, err)

	for _, j := range []*ActionRunJob{outer, inner} {
		got := unittest.AssertExistsAndLoadBean(t, &ActionRunJob{ID: j.ID})
		assert.Equal(t, StatusCancelled, got.Status, "job %q should be cancelled", j.JobID)
	}
	gotAttempt := unittest.AssertExistsAndLoadBean(t, &ActionRunAttempt{ID: attempt.ID})
	assert.Equal(t, StatusCancelled, gotAttempt.Status, "attempt must aggregate to Cancelled")
	gotRun := unittest.AssertExistsAndLoadBean(t, &ActionRun{ID: run.ID})
	assert.Equal(t, StatusCancelled, gotRun.Status, "run must aggregate to Cancelled, not stay Blocked")
}

func TestCancelJobs_RunWithoutCancellableJobs(t *testing.T) {
	// A run whose jobs all reached a final status while its own row stayed unfinished.
	// Cancelling has to refresh it anyway, or the run can never finish and can never be deleted either.

	newStuckRun := func(t *testing.T, index int64, withAttempt bool) (*ActionRun, []*ActionRunJob) {
		t.Helper()
		ctx := t.Context()

		run := &ActionRun{
			Title:         "stuck-waiting",
			RepoID:        4,
			Index:         index,
			OwnerID:       1,
			WorkflowID:    "test.yaml",
			TriggerUserID: 1,
			Ref:           "refs/heads/master",
			CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
			Event:         "push",
			TriggerEvent:  "push",
			EventPayload:  "{}",
			Status:        StatusWaiting,
		}
		require.NoError(t, db.Insert(ctx, run))

		var runAttemptID int64
		if withAttempt {
			attempt := &ActionRunAttempt{RepoID: run.RepoID, RunID: run.ID, Attempt: 1, TriggerUserID: 1, Status: StatusWaiting}
			require.NoError(t, db.Insert(ctx, attempt))
			run.LatestAttemptID = attempt.ID
			require.NoError(t, UpdateRun(ctx, run, "latest_attempt_id"))
			runAttemptID = attempt.ID
		}

		job := &ActionRunJob{
			RunID:        run.ID,
			RunAttemptID: runAttemptID,
			RepoID:       run.RepoID,
			OwnerID:      run.OwnerID,
			CommitSHA:    run.CommitSHA,
			Name:         "job1",
			JobID:        "job1",
			Attempt:      1,
			Status:       StatusSuccess,
			Stopped:      timeutil.TimeStampNow(),
		}
		require.NoError(t, db.Insert(ctx, job))
		return run, []*ActionRunJob{job}
	}

	t.Run("attempt", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		run, jobs := newStuckRun(t, 9801, true)

		cancelled, err := CancelJobs(t.Context(), jobs)
		require.NoError(t, err)
		assert.Empty(t, cancelled, "the job is already done, so there is nothing to cancel")

		gotAttempt := unittest.AssertExistsAndLoadBean(t, &ActionRunAttempt{ID: run.LatestAttemptID})
		assert.Equal(t, StatusSuccess, gotAttempt.Status)
		gotRun := unittest.AssertExistsAndLoadBean(t, &ActionRun{ID: run.ID})
		assert.Equal(t, StatusSuccess, gotRun.Status)
		assert.NotZero(t, gotRun.Stopped)
	})

	// Runs created before migration v331 have no attempt, their status lives on the run row itself.
	t.Run("legacy run without attempt", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		run, jobs := newStuckRun(t, 9802, false)

		cancelled, err := CancelJobs(t.Context(), jobs)
		require.NoError(t, err)
		assert.Empty(t, cancelled)

		gotRun := unittest.AssertExistsAndLoadBean(t, &ActionRun{ID: run.ID})
		assert.Equal(t, StatusSuccess, gotRun.Status)
		assert.NotZero(t, gotRun.Stopped)
	})
}

func TestParseJobDeferredMatrixPlaceholder(t *testing.T) {
	// A placeholder is persisted with the raw matrix and without its needs, so routing its payload
	// through jobparser.Parse re-expands that matrix. The job emitter reads `if:` (and so ParseJob)
	// before it can expand the placeholder, and it only logs a parse failure: getting this wrong
	// leaves the job Blocked on every pass, the run never finishes and its concurrency group is
	// never released.
	payload := func(matrix string) []byte {
		return fmt.Appendf(nil, `name: test
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        %s
    steps:
      - run: echo
`, matrix)
	}

	// The shape Parse happens to survive, so only the name tells the two paths apart. What Parse makes
	// of every other shape is asserted in TestParseRawSingleWorkflowRoundTripsDeferredPlaceholder.
	t.Run("a placeholder is read back, not re-expanded", func(t *testing.T) {
		job := &ActionRunJob{ID: 1, JobID: "build", IsMatrixDeferred: true, WorkflowPayload: payload("version: ${{ fromJson(needs.setup.outputs.m) }}")}
		parsed, err := job.ParseJob()
		require.NoError(t, err)
		require.NotNil(t, parsed)
		assert.Equal(t, "build", parsed.Name)
	})

	t.Run("an expanded job still goes through the full parse", func(t *testing.T) {
		job := &ActionRunJob{ID: 1, JobID: "build", WorkflowPayload: payload("version: [1]")}
		parsed, err := job.ParseJob()
		require.NoError(t, err)
		require.NotNil(t, parsed)
		// Parse bakes the combination into the name, ParseRawSingleWorkflow would not.
		assert.Equal(t, "build (1)", parsed.Name)
	})
}
