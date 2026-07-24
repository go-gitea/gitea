// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"math"
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStopCodespaceQueuesUserStopAndTakesQueuedIdleStop(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceUUID := "67676767-6767-4676-8676-676767676767"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 15,
	})
	result, err := StopCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceUUID: codespaceUUID})
	require.NoError(t, err)
	assert.Equal(t, codespace_model.StatusRunning, result.Status)
	assert.Equal(t, codespace_model.OperationStop, result.OperationType)
	assert.EqualValues(t, 16, result.OperationRVersion)
	row := loadServiceCodespace(t, codespaceUUID)
	assert.Equal(t, codespace_model.OperationStatusQueued, row.OperationStatus)
	assert.Equal(t, codespace_model.OperationTriggerUser, row.OperationTrigger)

	idleUUID := "68686868-6868-4686-8686-686868686868"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 idleUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    17,
		OperationType:        codespace_model.OperationStop,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerIdle,
		OperationCreatedUnix: time.Now().Unix(),
	})
	updatedUnix := loadServiceCodespace(t, idleUUID).UpdatedUnix
	result, err = StopCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceUUID: idleUUID})
	require.NoError(t, err)
	assert.EqualValues(t, 17, result.OperationRVersion)
	row = loadServiceCodespace(t, idleUUID)
	assert.Equal(t, codespace_model.OperationTriggerUser, row.OperationTrigger)
	assert.Equal(t, updatedUnix, row.UpdatedUnix)
}

func TestStopCodespaceRejectsActiveUserStop(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	queuedUUID := "68686868-6868-4686-8686-686868686869"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 queuedUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    17,
		OperationType:        codespace_model.OperationStop,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	_, err := StopCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceUUID: queuedUUID})
	require.ErrorIs(t, err, ErrLifecycleActionStateUnavailable)

	runningUUID := "68686868-6868-4686-8686-686868686870"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  runningUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     18,
		OperationType:         codespace_model.OperationStop,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Unix(),
		OperationStartedUnix:  time.Now().Unix(),
		OperationDeadlineUnix: time.Now().Add(time.Minute).Unix(),
	})
	_, err = StopCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceUUID: runningUUID})
	require.ErrorIs(t, err, ErrLifecycleActionStateUnavailable)
}

func TestResumeCodespaceQueuesResume(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceUUID := "69696969-6969-4696-8696-696969696969"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusStopped,
		OperationRVersion:     18,
		InteractionGeneration: 7,
	})
	result, err := ResumeCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceUUID: codespaceUUID})
	require.NoError(t, err)
	assert.Equal(t, codespace_model.StatusStopped, result.Status)
	assert.Equal(t, codespace_model.OperationResume, result.OperationType)
	assert.EqualValues(t, 19, result.OperationRVersion)
	row := loadServiceCodespace(t, codespaceUUID)
	assert.Equal(t, codespace_model.OperationStatusQueued, row.OperationStatus)
	assert.Equal(t, codespace_model.OperationTriggerUser, row.OperationTrigger)
	assert.EqualValues(t, 8, row.InteractionGeneration)
	assert.Positive(t, row.LastActiveUnix)
}

func TestDeleteCodespacePhysicalForUnboundCreatingAndFailed(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	creatingUUID := "70707070-7070-4707-8707-707070707070"
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		UUID:                 creatingUUID,
		Status:               codespace_model.StatusCreating,
		OperationRVersion:    20,
		OperationType:        codespace_model.OperationCreate,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	result, err := DeleteCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceUUID: creatingUUID})
	require.NoError(t, err)
	assert.True(t, result.Deleted)
	assertServiceNotExists(t, new(codespace_model.Codespace), "uuid = ?", creatingUUID)

	failedUUID := "71717171-7171-4717-8717-717171717171"
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		UUID:   failedUUID,
		Status: codespace_model.StatusFailed,
	})
	insertServiceCredentials(t, failedUUID)
	result, err = DeleteCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceUUID: failedUUID})
	require.NoError(t, err)
	assert.True(t, result.Deleted)
	assertServiceNotExists(t, new(codespace_model.Codespace), "uuid = ?", failedUUID)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", failedUUID)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", failedUUID)
}

func TestDeleteUnboundCodespaceRequiresCurrentRow(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	codespaceUUID := "79797979-7979-4797-8797-797979797979"
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusFailed,
		OperationRVersion: 20,
	})
	stale := loadServiceCodespace(t, codespaceUUID)
	row := loadServiceCodespace(t, codespaceUUID)
	row.OperationRVersion = 21
	_, err := unittest.GetXORMEngine().ID(codespaceUUID).Cols("operation_r_version").Update(row)
	require.NoError(t, err)

	deleted, err := deleteUnboundCodespaceIfCurrent(t.Context(), stale)
	require.NoError(t, err)
	assert.False(t, deleted)
	assertServiceExists(t, new(codespace_model.Codespace), "uuid = ?", codespaceUUID)
}

func TestDeleteCodespaceQueuesBoundDeleteAndReplacesOperation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceUUID := "72727272-7272-4727-8727-727272727272"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     21,
		OperationType:         codespace_model.OperationStop,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Unix(),
		OperationStartedUnix:  time.Now().Unix(),
		OperationDeadlineUnix: time.Now().Add(time.Minute).Unix(),
	})
	insertServiceCredentials(t, codespaceUUID)

	result, err := DeleteCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceUUID: codespaceUUID})
	require.NoError(t, err)
	assert.False(t, result.Deleted)
	assert.Equal(t, codespace_model.StatusDeleting, result.Status)
	assert.Equal(t, codespace_model.OperationDelete, result.OperationType)
	assert.EqualValues(t, 22, result.OperationRVersion)
	row := loadServiceCodespace(t, codespaceUUID)
	assert.Equal(t, codespace_model.StatusDeleting, row.Status)
	assert.Equal(t, codespace_model.OperationDelete, row.OperationType)
	assert.Equal(t, codespace_model.OperationStatusQueued, row.OperationStatus)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", codespaceUUID)
}

func TestLifecycleActionValidation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	runningUUID := "73737373-7373-4737-8737-737373737373"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 runningUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    math.MaxInt64,
		OperationType:        codespace_model.OperationDelete,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	_, err := ResumeCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceUUID: runningUUID})
	require.ErrorIs(t, err, ErrLifecycleActionStateUnavailable)
	_, err = DeleteCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceUUID: runningUUID})
	require.ErrorIs(t, err, ErrLifecycleActionVersionExhausted)
}
