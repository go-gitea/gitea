// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testArtifactName = "autotest-build"
	testArtifactPath = ".tmp/build-artifact"
)

// newArtifactTestTask inserts a run/attempt/job and returns an ActionTask
// (with its Job preloaded) that CreateArtifact can be called with.
func newArtifactTestTask(t *testing.T, runIndex int64) *ActionTask {
	t.Helper()
	ctx := t.Context()

	run := &ActionRun{
		Title:         "artifact-test",
		RepoID:        4,
		Index:         runIndex,
		OwnerID:       1,
		WorkflowID:    "artifact-test.yaml",
		TriggerUserID: 1,
		Ref:           "refs/heads/master",
		CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:         "push",
		TriggerEvent:  "push",
		EventPayload:  "{}",
		Status:        StatusRunning,
	}
	require.NoError(t, db.Insert(ctx, run))

	attempt := &ActionRunAttempt{
		RepoID:        run.RepoID,
		RunID:         run.ID,
		Attempt:       1,
		TriggerUserID: 1,
		Status:        StatusRunning,
	}
	require.NoError(t, db.Insert(ctx, attempt))

	job := &ActionRunJob{
		RunID:        run.ID,
		RunAttemptID: attempt.ID,
		RepoID:       run.RepoID,
		OwnerID:      run.OwnerID,
		CommitSHA:    run.CommitSHA,
		Name:         "build",
		JobID:        "build",
		Attempt:      1,
		Status:       StatusRunning,
	}
	require.NoError(t, db.Insert(ctx, job))

	return &ActionTask{
		RepoID:    run.RepoID,
		OwnerID:   run.OwnerID,
		CommitSHA: run.CommitSHA,
		Job:       job,
	}
}

func TestCreateArtifact_NewArtifact(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	task := newArtifactTestTask(t, 9601)

	art, err := CreateArtifact(ctx, task, testArtifactName, testArtifactPath, 3)
	require.NoError(t, err)
	assert.Equal(t, ArtifactStatusUploadPending, art.Status)
	assert.WithinDuration(t, time.Now().AddDate(0, 0, 3), art.ExpiredUnix.AsLocalTime(), time.Minute)
}

// TestCreateArtifact_ReuploadAfterExpiry reproduces the bug where re-running a
// job whose artifact had already expired (and had its storage blob deleted by
// the cleanup job) resurrects the same DB row by only bumping expired_unix,
// while leaving status stuck at Expired/PendingDeletion/Deleted and the stale
// StoragePath/FileSize in place. A fresh upload must instead reinitialize the
// row as a brand-new pending upload.
func TestCreateArtifact_ReuploadAfterExpiry(t *testing.T) {
	terminalStatuses := []ArtifactStatus{
		ArtifactStatusExpired,
		ArtifactStatusPendingDeletion,
		ArtifactStatusDeleted,
	}

	for i, staleStatus := range terminalStatuses {
		t.Run(staleStatus.ToString(), func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			ctx := t.Context()

			task := newArtifactTestTask(t, 9610+int64(i))

			// Simulate a previously-uploaded, now-expired-and-deleted artifact
			// for the exact same run/attempt/name/path.
			stale := &ActionArtifact{
				ArtifactName:          testArtifactName,
				ArtifactPath:          testArtifactPath,
				RunID:                 task.Job.RunID,
				RunAttemptID:          task.Job.RunAttemptID,
				RunnerID:              1,
				RepoID:                task.RepoID,
				OwnerID:               task.OwnerID,
				CommitSHA:             task.CommitSHA,
				StoragePath:           "old/deleted/blob/path",
				FileSize:              123456,
				FileCompressedSize:    123456,
				ContentEncodingOrType: "application/zip",
				Status:                staleStatus,
				ExpiredUnix:           timeutil.TimeStamp(time.Now().AddDate(0, 0, -1).Unix()),
			}
			require.NoError(t, db.Insert(ctx, stale))

			// Backdate the creation time (raw SQL: xorm won't ORM-update a
			// "created" column) so we can assert it survives the re-upload.
			origCreated := timeutil.TimeStamp(time.Now().AddDate(0, 0, -30).Unix())
			_, err := db.GetEngine(ctx).Exec("UPDATE action_artifact SET created_unix = ? WHERE id = ?", origCreated, stale.ID)
			require.NoError(t, err)

			art, err := CreateArtifact(ctx, task, testArtifactName, testArtifactPath, 3)
			require.NoError(t, err)

			// Re-upload must revive the row as a fresh pending upload, not
			// just extend the expiry on an already-dead record.
			assert.Equal(t, ArtifactStatusUploadPending, art.Status)
			assert.Empty(t, art.StoragePath)
			assert.Zero(t, art.FileSize)
			assert.Zero(t, art.FileCompressedSize)
			assert.WithinDuration(t, time.Now().AddDate(0, 0, 3), art.ExpiredUnix.AsLocalTime(), time.Minute)

			// The unique index on (run_id, run_attempt_id, name, path) means
			// there must still only be exactly one row for this key.
			count, err := db.GetEngine(ctx).Where(
				"run_id = ? AND run_attempt_id = ? AND artifact_name = ? AND artifact_path = ?",
				task.Job.RunID, task.Job.RunAttemptID, testArtifactName, testArtifactPath,
			).Count(new(ActionArtifact))
			require.NoError(t, err)
			assert.EqualValues(t, 1, count)

			// And the DB row itself (not just the returned struct) must reflect the revival.
			reloaded := unittest.AssertExistsAndLoadBean(t, &ActionArtifact{ID: stale.ID})
			assert.Equal(t, ArtifactStatusUploadPending, reloaded.Status)
			assert.Empty(t, reloaded.StoragePath)

			// The returned struct is what callers hand straight back to an
			// AllCols() UpdateArtifactByID; that must not wipe created_unix.
			require.Equal(t, origCreated, art.CreatedUnix)
			art.ContentEncodingOrType = "gzip"
			require.NoError(t, UpdateArtifactByID(ctx, art.ID, art))
			reloaded = unittest.AssertExistsAndLoadBean(t, &ActionArtifact{ID: stale.ID})
			assert.Equal(t, origCreated, reloaded.CreatedUnix, "original creation time must survive the caller's update")
		})
	}
}

// TestCreateArtifact_ReuseInProgressUpload verifies chunked-upload progress
// (multiple CreateArtifact calls while status is still Pending) keeps
// reusing the same row and only refreshes expired_unix, since the artifact
// hasn't reached a terminal state yet.
func TestCreateArtifact_ReuseInProgressUpload(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	task := newArtifactTestTask(t, 9620)

	first, err := CreateArtifact(ctx, task, testArtifactName, testArtifactPath, 3)
	require.NoError(t, err)

	// Simulate the runner having already written some content for this
	// in-progress upload.
	first.StoragePath = "in-progress/path"
	first.FileSize = 42
	require.NoError(t, UpdateArtifactByID(ctx, first.ID, first))

	second, err := CreateArtifact(ctx, task, testArtifactName, testArtifactPath, 3)
	require.NoError(t, err)

	assert.Equal(t, first.ID, second.ID)
	reloaded := unittest.AssertExistsAndLoadBean(t, &ActionArtifact{ID: first.ID})
	assert.Equal(t, "in-progress/path", reloaded.StoragePath)
	assert.EqualValues(t, 42, reloaded.FileSize)
}
