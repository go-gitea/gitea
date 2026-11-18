// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"fmt"
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconcileCodespacesAppliesRunningTimeoutsAndRetention(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	now := time.Now().Unix()
	queuedID := int64(1019)
	runningID := int64(1069)
	runningStopID := int64(1098)
	failedID := int64(1076)

	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   queuedID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    3,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: now - 3600,
	})
	stopCreatedUnix := now - 120
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID: runningStopID, Status: codespace_model.StatusRunning, OperationRVersion: 6,
		OperationTrigger: codespace_model.OperationTriggerUser, OperationCreatedUnix: stopCreatedUnix,
		OperationStartedUnix: now - 60, OperationDeadlineUnix: now - 1,
	})
	insertServiceCredentials(t, queuedID)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    runningID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     4,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  1,
		OperationStartedUnix:  now - int64(setting.Codespace.OperationMaxDuration/time.Second) - 1,
		OperationDeadlineUnix: now - 1,
	})
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:          failedID,
		Status:      codespace_model.StatusFailed,
		UpdatedUnix: now - int64((2*time.Hour)/time.Second),
	})
	_, err := db.GetEngine(t.Context()).Where("id = ?", failedID).Cols("updated_unix").Update(&codespace_model.Codespace{
		UpdatedUnix: now - int64((2*time.Hour)/time.Second),
	})
	require.NoError(t, err)
	insertServiceCredentials(t, failedID)
	unassignedFailed := &codespace_model.Codespace{
		UserID: 1, RepoID: 2, RefType: "branch", RefName: "main", EnvironmentTag: "default",
		CommitSHA:           "0123456789abcdef0123456789abcdef01234567",
		DevContainerContent: `{"image":"mcr.microsoft.com/devcontainers/base:ubuntu"}`, Status: codespace_model.StatusFailed,
		OperationRVersion: 5, AutoStopMode: codespace_model.AutoStopModeDefault, CreatedUnix: 1,
		UpdatedUnix: now - int64((2*time.Hour)/time.Second),
	}
	_, err = db.GetEngine(t.Context()).Table(unassignedFailed).Insert(unassignedFailed)
	require.NoError(t, err)
	_, err = db.GetEngine(t.Context()).ID(unassignedFailed.ID).Cols("updated_unix").Update(&codespace_model.Codespace{UpdatedUnix: unassignedFailed.UpdatedUnix})
	require.NoError(t, err)
	unassignedQueued := *unassignedFailed
	unassignedQueued.ID = 0
	unassignedQueued.Status = codespace_model.StatusCreating
	unassignedQueued.OperationCreatedUnix = now - 3600
	unassignedQueued.OperationTrigger = codespace_model.OperationTriggerUser
	_, err = db.GetEngine(t.Context()).Table(&unassignedQueued).Insert(&unassignedQueued)
	require.NoError(t, err)
	claimedCreate := unassignedQueued
	claimedCreate.ID = 0
	claimedCreate.ManagerID = manager.ID
	claimedCreate.OperationStartedUnix = now - 60
	claimedCreate.OperationDeadlineUnix = now - 1
	_, err = db.GetEngine(t.Context()).Table(&claimedCreate).Insert(&claimedCreate)
	require.NoError(t, err)

	result, err := ReconcileCodespaces(t.Context(), ReconcileCodespacesOptions{FailedOlderThan: time.Hour})
	require.NoError(t, err)
	assert.Equal(t, 3, result.RunningTimedOut)
	assert.Equal(t, 1, result.FailedDeleted)
	var waiting codespace_model.Codespace
	has, err := db.GetEngine(t.Context()).ID(unassignedQueued.ID).Get(&waiting)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, codespace_model.StatusCreating, waiting.Status)
	assert.True(t, codespace_model.IsOperationQueued(&waiting))
	var timedOut codespace_model.Codespace
	has, err = db.GetEngine(t.Context()).ID(claimedCreate.ID).Get(&timedOut)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, codespace_model.StatusFailed, timedOut.Status)
	assert.False(t, hasActiveOperation(&timedOut))

	queued := loadServiceCodespace(t, queuedID)
	assert.Equal(t, codespace_model.StatusRunning, queued.Status)
	assert.True(t, codespace_model.IsOperationQueued(queued))
	assertServiceExists(t, new(codespace_model.GiteaToken), "codespace_id = ?", queuedID)
	assertServiceExists(t, new(codespace_model.SSHKey), "codespace_id = ?", queuedID)

	running := loadServiceCodespace(t, runningID)
	assert.Equal(t, codespace_model.StatusFailed, running.Status)
	assert.False(t, hasActiveOperation(running))
	retriedStop := loadServiceCodespace(t, runningStopID)
	assert.Equal(t, codespace_model.StatusRunning, retriedStop.Status)
	assert.EqualValues(t, 7, retriedStop.OperationRVersion)
	assert.True(t, codespace_model.IsOperationQueued(retriedStop))
	assert.Equal(t, stopCreatedUnix, retriedStop.OperationCreatedUnix)
	assert.Zero(t, retriedStop.OperationStartedUnix)
	assert.Zero(t, retriedStop.OperationDeadlineUnix)

	assertServiceExists(t, new(codespace_model.Codespace), "id = ?", failedID)
	assertServiceExists(t, new(codespace_model.GiteaToken), "codespace_id = ?", failedID)
	assertServiceExists(t, new(codespace_model.SSHKey), "codespace_id = ?", failedID)
	assertServiceNotExists(t, new(codespace_model.Codespace), "id = ?", unassignedFailed.ID)
}

func TestReconcileCodespacesRequiresPositiveRetention(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	result, err := ReconcileCodespaces(t.Context(), ReconcileCodespacesOptions{})
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestReconcileCodespacesCleansExpiredTokensAndKeepsStableRows(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceID := int64(1099)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:          codespaceID,
		Status:      codespace_model.StatusRunning,
		UpdatedUnix: 1,
	})
	now := time.Now().Unix()
	for i := range reconcileCodespacesBatchSize + 1 {
		require.NoError(t, db.Insert(t.Context(), &codespace_model.OpenToken{
			CodeHash: fmt.Sprintf("%064x", i), ExpiresUnix: now - 60,
		}))
	}
	validHash := openTokenCodeHash(generateOpenTokenCode())
	require.NoError(t, db.Insert(t.Context(), &codespace_model.OpenToken{CodeHash: validHash, ExpiresUnix: now + 3600}))

	result, err := ReconcileCodespaces(t.Context(), ReconcileCodespacesOptions{FailedOlderThan: time.Hour})
	require.NoError(t, err)
	assert.Equal(t, &ReconcileCodespacesResult{OpenTokensDeleted: reconcileCodespacesBatchSize}, result)
	result, err = ReconcileCodespaces(t.Context(), ReconcileCodespacesOptions{FailedOlderThan: time.Hour})
	require.NoError(t, err)
	assert.Equal(t, &ReconcileCodespacesResult{OpenTokensDeleted: 1}, result)
	assertServiceExists(t, new(codespace_model.OpenToken), "code_hash = ?", validHash)
	var count int64
	count, err = db.GetEngine(t.Context()).Where("id = ?", codespaceID).Count(new(codespace_model.Codespace))
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}
