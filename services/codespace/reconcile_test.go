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
	failedUUID := "14141414-1414-4414-8414-141414141414"
	freshFailedUUID := "15151515-1515-4515-8515-151515151515"

	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 queuedUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    3,
		OperationType:        codespace_model.OperationStop,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: now - int64(setting.Codespace.QueueTimeout/time.Second) - 1,
	})
	insertServiceCredentials(t, queuedUUID)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  runningUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     4,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
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
	_, err := db.GetEngine(t.Context()).ID(failedUUID).Cols("updated_unix").Update(&codespace_model.Codespace{
		UpdatedUnix: now - int64((2*time.Hour)/time.Second),
	})
	require.NoError(t, err)
	insertServiceCredentials(t, failedUUID)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:        freshFailedUUID,
		Status:      codespace_model.StatusFailed,
		UpdatedUnix: now,
	})
	_, err = db.GetEngine(t.Context()).ID(freshFailedUUID).Cols("updated_unix").Update(&codespace_model.Codespace{UpdatedUnix: now})
	require.NoError(t, err)

	result, err := ReconcileCodespaces(t.Context(), ReconcileCodespacesOptions{FailedOlderThan: time.Hour})
	require.NoError(t, err)
	assert.Equal(t, 1, result.QueuedTimedOut)
	assert.Equal(t, 1, result.RunningTimedOut)
	assert.Equal(t, 1, result.FailedDeleted)

	queued := loadServiceCodespace(t, queuedUUID)
	assert.Equal(t, codespace_model.StatusRunning, queued.Status)
	assert.Empty(t, queued.OperationStatus)
	assertServiceExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", queuedUUID)
	assertServiceExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", queuedUUID)

	running := loadServiceCodespace(t, runningUUID)
	assert.Equal(t, codespace_model.StatusFailed, running.Status)
	assert.Empty(t, running.OperationStatus)

	assertServiceNotExists(t, new(codespace_model.Codespace), "uuid = ?", failedUUID)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", failedUUID)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", failedUUID)
	assertServiceExists(t, new(codespace_model.Codespace), "uuid = ?", freshFailedUUID)
}

func TestReconcileCodespacesRequiresPositiveRetention(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	result, err := ReconcileCodespaces(t.Context(), ReconcileCodespacesOptions{})
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestReconcileCodespacesSkipsChangedRows(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceUUID := "16161616-1616-4616-8616-161616161616"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:            codespaceUUID,
		Status:          codespace_model.StatusRunning,
		OperationStatus: codespace_model.OperationStatusQueued,
		UpdatedUnix:     1,
	})

	result, err := ReconcileCodespaces(t.Context(), ReconcileCodespacesOptions{FailedOlderThan: time.Hour})
	require.NoError(t, err)
	assert.Zero(t, result.QueuedTimedOut)

	var count int64
	count, err = db.GetEngine(t.Context()).Where("uuid = ?", codespaceUUID).Count(new(codespace_model.Codespace))
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}
