// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconcileCodespacesAppliesTimeoutsAndRetention(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	now := time.Now().Unix()
	queuedUUID := "12121212-1212-4212-8212-121212121212"
	runningUUID := "13131313-1313-4313-8313-131313131313"
	runningStopUUID := "13131313-1313-4313-8313-131313131314"
	failedUUID := "14141414-1414-4414-8414-141414141414"

	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 queuedUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    3,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: now - int64(setting.Codespace.QueueTimeout/time.Second) - 1,
	})
	stopCreatedUnix := now - 120
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID: runningStopUUID, Status: codespace_model.StatusRunning, OperationRVersion: 6,
		OperationTrigger: codespace_model.OperationTriggerUser, OperationCreatedUnix: stopCreatedUnix,
		OperationStartedUnix: now - 60, OperationDeadlineUnix: now - 1,
	})
	insertServiceCredentials(t, queuedUUID)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  runningUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     4,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  1,
		OperationStartedUnix:  now - int64(setting.Codespace.OperationMaxDuration/time.Second) - 1,
		OperationDeadlineUnix: now - 1,
	})
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:        failedUUID,
		Status:      codespace_model.StatusFailed,
		UpdatedUnix: now - int64((2*time.Hour)/time.Second),
	})
	_, err := db.GetEngine(t.Context()).Where("uuid = ?", failedUUID).Cols("updated_unix").Update(&codespace_model.Codespace{
		UpdatedUnix: now - int64((2*time.Hour)/time.Second),
	})
	require.NoError(t, err)
	insertServiceCredentials(t, failedUUID)
	unboundFailed := &codespace_model.Codespace{
		UserID: 1, RepoID: 2, RefType: "branch", RefName: "main", EnvironmentTag: "default",
		CommitSHA:           "0123456789abcdef0123456789abcdef01234567",
		DevContainerContent: `{"image":"mcr.microsoft.com/devcontainers/base:ubuntu"}`, Status: codespace_model.StatusFailed,
		OperationRVersion: 5, AutoStopMode: codespace_model.AutoStopModeDefault, CreatedUnix: 1,
		UpdatedUnix: now - int64((2*time.Hour)/time.Second),
	}
	_, err = db.GetEngine(t.Context()).Table(unboundFailed).Omit("uuid").Insert(unboundFailed)
	require.NoError(t, err)
	_, err = db.GetEngine(t.Context()).ID(unboundFailed.ID).Cols("updated_unix").Update(&codespace_model.Codespace{UpdatedUnix: unboundFailed.UpdatedUnix})
	require.NoError(t, err)

	result, err := ReconcileCodespaces(t.Context(), ReconcileCodespacesOptions{FailedOlderThan: time.Hour})
	require.NoError(t, err)
	assert.Zero(t, result.QueuedTimedOut)
	assert.Equal(t, 2, result.RunningTimedOut)
	assert.Equal(t, 1, result.FailedDeleted)

	queued := loadServiceCodespace(t, queuedUUID)
	assert.Equal(t, codespace_model.StatusRunning, queued.Status)
	assert.True(t, codespace_model.IsOperationQueued(queued))
	assertServiceExists(t, new(codespace_model.GiteaToken), "codespace_id = (SELECT id FROM codespace WHERE uuid = ?)", queuedUUID)
	assertServiceExists(t, new(codespace_model.SSHKey), "codespace_id = (SELECT id FROM codespace WHERE uuid = ?)", queuedUUID)

	running := loadServiceCodespace(t, runningUUID)
	assert.Equal(t, codespace_model.StatusFailed, running.Status)
	assert.False(t, hasActiveOperation(running))
	retriedStop := loadServiceCodespace(t, runningStopUUID)
	assert.Equal(t, codespace_model.StatusRunning, retriedStop.Status)
	assert.EqualValues(t, 7, retriedStop.OperationRVersion)
	assert.True(t, codespace_model.IsOperationQueued(retriedStop))
	assert.Equal(t, stopCreatedUnix, retriedStop.OperationCreatedUnix)
	assert.Zero(t, retriedStop.OperationStartedUnix)
	assert.Zero(t, retriedStop.OperationDeadlineUnix)

	assertServiceExists(t, new(codespace_model.Codespace), "uuid = ?", failedUUID)
	assertServiceExists(t, new(codespace_model.GiteaToken), "codespace_id = (SELECT id FROM codespace WHERE uuid = ?)", failedUUID)
	assertServiceExists(t, new(codespace_model.SSHKey), "codespace_id = (SELECT id FROM codespace WHERE uuid = ?)", failedUUID)
	assertServiceNotExists(t, new(codespace_model.Codespace), "id = ?", unboundFailed.ID)
}

func TestReconcileCodespacesRequiresPositiveRetention(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	result, err := ReconcileCodespaces(t.Context(), ReconcileCodespacesOptions{})
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestReconcileCodespacesLeavesStableRowsUnchanged(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceUUID := "16161616-1616-4616-8616-161616161616"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:        codespaceUUID,
		Status:      codespace_model.StatusRunning,
		UpdatedUnix: 1,
	})

	result, err := ReconcileCodespaces(t.Context(), ReconcileCodespacesOptions{FailedOlderThan: time.Hour})
	require.NoError(t, err)
	assert.Zero(t, result.QueuedTimedOut)

	var count int64
	count, err = db.GetEngine(t.Context()).Where("uuid = ?", codespaceUUID).Count(new(codespace_model.Codespace))
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}
