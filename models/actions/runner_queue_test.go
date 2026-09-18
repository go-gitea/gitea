// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListRunnerRepoQueues(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	const label = "queue-test-label"
	// The actions package fixture set does not load repo_unit.yml.
	enableActionsUnit(t, 1)
	enableActionsUnit(t, 3)
	enableActionsUnit(t, 4)

	// user2 owns repo1 (actions enabled here) and repo2 (left without Actions).
	ownerRunner := insertQueueRunner(t, "11111111-1111-4111-8111-000000000001", 2, 0, label)
	queues, err := ListRunnerRepoQueues(t.Context(), ownerRunner)
	require.NoError(t, err)
	require.Len(t, queues, 1)
	assert.Equal(t, int64(1), queues[0].Repo.ID)
	assert.Equal(t, 0, queues[0].Waiting)
	assert.Equal(t, "/user2/repo1/actions", queues[0].ActionsLink())

	insertQueueJob(t, 1, 2, StatusWaiting, []string{label}, false)
	insertQueueJob(t, 1, 2, StatusWaiting, []string{"other-label"}, false)
	insertQueueJob(t, 1, 2, StatusSuccess, []string{label}, false)
	insertQueueJob(t, 1, 2, StatusBlocked, []string{label}, false)
	insertQueueJob(t, 1, 2, StatusWaiting, []string{label}, true)  // reusable caller is not pickable
	insertQueueJob(t, 3, 3, StatusWaiting, []string{label}, false) // other owner

	queues, err = ListRunnerRepoQueues(t.Context(), ownerRunner)
	require.NoError(t, err)
	require.Len(t, queues, 1)
	assert.Equal(t, int64(1), queues[0].Repo.ID)
	assert.Equal(t, 1, queues[0].Waiting)

	// A repository runner only sees its own repository, even when that count is zero.
	repoRunner := insertQueueRunner(t, "11111111-1111-4111-8111-000000000002", 0, 4, label)
	queues, err = ListRunnerRepoQueues(t.Context(), repoRunner)
	require.NoError(t, err)
	require.Len(t, queues, 1)
	assert.Equal(t, int64(4), queues[0].Repo.ID)
	assert.Equal(t, 0, queues[0].Waiting)

	insertQueueJob(t, 4, 5, StatusWaiting, []string{label}, false)
	insertQueueJob(t, 4, 5, StatusWaiting, []string{label, "also-required"}, false) // runner lacks also-required

	queues, err = ListRunnerRepoQueues(t.Context(), repoRunner)
	require.NoError(t, err)
	require.Len(t, queues, 1)
	assert.Equal(t, 1, queues[0].Waiting)
	assert.Equal(t, "/user5/repo4/actions", queues[0].ActionsLink())

	// A global runner is not a list of every repository. Only rows with a waiting job.
	insertQueueJob(t, 1, 2, StatusWaiting, []string{label}, false)
	globalRunner := insertQueueRunner(t, "11111111-1111-4111-8111-000000000003", 0, 0, label)
	queues, err = ListRunnerRepoQueues(t.Context(), globalRunner)
	require.NoError(t, err)
	require.Len(t, queues, 3)
	assert.Equal(t, int64(1), queues[0].Repo.ID) // user2/repo1 has 2
	assert.Equal(t, 2, queues[0].Waiting)
	assert.Equal(t, 1, queues[1].Waiting)
	assert.Equal(t, 1, queues[2].Waiting)
	assert.Equal(t, "org3/repo3", queues[1].Repo.FullName())
	assert.Equal(t, "user5/repo4", queues[2].Repo.FullName())
}

func TestFindRunningTasksByRunnerIDs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	runner := insertQueueRunner(t, "22222222-2222-4222-8222-000000000001", 2, 0, "queue-test-label")
	other := insertQueueRunner(t, "22222222-2222-4222-8222-000000000002", 2, 0, "queue-test-label")

	running := &ActionTask{RunnerID: runner.ID, Status: StatusRunning, RepoID: 1, JobID: 1}
	running.GenerateAndFillToken()
	require.NoError(t, db.Insert(t.Context(), running))

	finished := &ActionTask{RunnerID: runner.ID, Status: StatusSuccess, RepoID: 1, JobID: 2}
	finished.GenerateAndFillToken()
	require.NoError(t, db.Insert(t.Context(), finished))

	found, err := FindRunningTasksByRunnerIDs(t.Context(), []int64{runner.ID, other.ID})
	require.NoError(t, err)
	require.Len(t, found[runner.ID], 1)
	assert.Equal(t, running.ID, found[runner.ID][0].ID)
	assert.Empty(t, found[other.ID])

	found, err = FindRunningTasksByRunnerIDs(t.Context(), nil)
	require.NoError(t, err)
	assert.Empty(t, found)
}

func enableActionsUnit(t *testing.T, repoID int64) {
	t.Helper()
	require.NoError(t, db.Insert(t.Context(), &repo_model.RepoUnit{
		RepoID: repoID,
		Type:   unit.TypeActions,
		Config: &repo_model.ActionsConfig{},
	}))
}

func insertQueueRunner(t *testing.T, uuid string, ownerID, repoID int64, label string) *ActionRunner {
	t.Helper()
	runner := &ActionRunner{
		UUID:        uuid,
		Name:        "queue-runner",
		OwnerID:     ownerID,
		RepoID:      repoID,
		AgentLabels: []string{label},
	}
	runner.GenerateAndFillToken()
	require.NoError(t, db.Insert(t.Context(), runner))
	return runner
}

func insertQueueJob(t *testing.T, repoID, ownerID int64, status Status, runsOn []string, reusable bool) {
	t.Helper()
	require.NoError(t, db.Insert(t.Context(), &ActionRunJob{
		RepoID:           repoID,
		OwnerID:          ownerID,
		Name:             "queue-job",
		JobID:            "queue-job",
		Attempt:          1,
		Status:           status,
		RunsOn:           runsOn,
		IsReusableCaller: reusable,
	}))
}
