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
	codespaceID := int64(1078)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 15,
	})
	result, err := StopCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceID: codespaceID})
	require.NoError(t, err)
	assert.Equal(t, codespace_model.StatusRunning, result.Status)
	assert.Equal(t, codespace_model.OperationStop, result.OperationType)
	assert.EqualValues(t, 16, result.OperationRVersion)
	row := loadServiceCodespace(t, codespaceID)
	assert.True(t, codespace_model.IsOperationQueued(row))
	assert.Equal(t, codespace_model.OperationTriggerUser, row.OperationTrigger)
	assert.EqualValues(t, 15, row.RuntimeOperationRVersion)

	idleID := int64(1079)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   idleID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    17,
		OperationTrigger:     codespace_model.OperationTriggerIdle,
		OperationCreatedUnix: time.Now().Unix(),
	})
	updatedUnix := loadServiceCodespace(t, idleID).UpdatedUnix
	result, err = StopCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceID: idleID})
	require.NoError(t, err)
	assert.EqualValues(t, 17, result.OperationRVersion)
	row = loadServiceCodespace(t, idleID)
	assert.Equal(t, codespace_model.OperationTriggerUser, row.OperationTrigger)
	assert.EqualValues(t, 17, row.RuntimeOperationRVersion)
	assert.Equal(t, updatedUnix, row.UpdatedUnix)
}

func TestStopCodespaceRejectsActiveUserStop(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	queuedID := int64(1080)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   queuedID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    17,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	_, err := StopCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceID: queuedID})
	require.ErrorIs(t, err, ErrLifecycleActionStateUnavailable)

	runningID := int64(1081)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    runningID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     18,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Unix(),
		OperationStartedUnix:  time.Now().Unix(),
		OperationDeadlineUnix: time.Now().Add(time.Minute).Unix(),
	})
	_, err = StopCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceID: runningID})
	require.ErrorIs(t, err, ErrLifecycleActionStateUnavailable)
}

func TestResumeCodespaceQueuesResume(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceID := int64(1082)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusStopped,
		OperationRVersion:     18,
		InteractionGeneration: 7,
	})
	result, err := ResumeCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceID: codespaceID})
	require.NoError(t, err)
	assert.Equal(t, codespace_model.StatusStopped, result.Status)
	assert.Equal(t, codespace_model.OperationResume, result.OperationType)
	assert.EqualValues(t, 19, result.OperationRVersion)
	row := loadServiceCodespace(t, codespaceID)
	assert.True(t, codespace_model.IsOperationQueued(row))
	assert.Equal(t, codespace_model.OperationTriggerUser, row.OperationTrigger)
	assert.EqualValues(t, 8, row.InteractionGeneration)
	assert.Positive(t, row.LastActiveUnix)

	result, err = CancelResumeCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceID: codespaceID})
	require.NoError(t, err)
	assert.Equal(t, codespace_model.StatusStopped, result.Status)
	assert.Empty(t, result.OperationType)
	assert.EqualValues(t, 19, result.OperationRVersion)
	row = loadServiceCodespace(t, codespaceID)
	assert.False(t, hasActiveOperation(row))
	assert.EqualValues(t, 8, row.InteractionGeneration)

	_, err = CancelResumeCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceID: codespaceID})
	require.ErrorIs(t, err, ErrLifecycleActionStateUnavailable)
}

func TestDeleteCodespacePhysicalForUnassignedCreatingAndFailed(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	creatingID := int64(1083)
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		ID:                   creatingID,
		Status:               codespace_model.StatusCreating,
		OperationRVersion:    20,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	result, err := DeleteCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceID: creatingID})
	require.NoError(t, err)
	assert.True(t, result.Deleted)
	assertServiceNotExists(t, new(codespace_model.Codespace), "id = ?", creatingID)

	failedID := int64(1084)
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		ID:     failedID,
		Status: codespace_model.StatusFailed,
	})
	insertServiceCredentials(t, failedID)
	openCode := generateOpenTokenCode()
	insertOpenToken(t, openCode, failedID, "workspace", 0, time.Now().Add(time.Minute).Unix())
	result, err = DeleteCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceID: failedID})
	require.NoError(t, err)
	assert.True(t, result.Deleted)
	assertServiceNotExists(t, new(codespace_model.Codespace), "id = ?", failedID)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_id = ?", failedID)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_id = ?", failedID)
	assertOpenTokenExists(t, openCode, false)
}

func TestDeleteUnassignedCodespaceRequiresCurrentRow(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	codespaceID := int64(1085)
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusFailed,
		OperationRVersion: 20,
	})
	stale := loadServiceCodespace(t, codespaceID)
	row := loadServiceCodespace(t, codespaceID)
	row.OperationRVersion = 21
	_, err := unittest.GetXORMEngine().Where("id = ?", codespaceID).Cols("operation_r_version").Update(row)
	require.NoError(t, err)

	err = deleteCodespaceRowForFinal(t.Context(), stale)
	require.ErrorIs(t, err, errCodespaceStateChanged)
	assertServiceExists(t, new(codespace_model.Codespace), "id = ?", codespaceID)
}

func TestDeleteCodespaceQueuesAssignedDeleteAndReplacesOperation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	for _, tc := range []struct {
		name   string
		id     int64
		status string
	}{
		{
			name:   "create",
			id:     int64(1086),
			status: codespace_model.StatusCreating,
		},
		{
			name:   "resume",
			id:     int64(1087),
			status: codespace_model.StatusStopped,
		},
		{
			name:   "stop",
			id:     int64(1088),
			status: codespace_model.StatusRunning,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
				ID:                    tc.id,
				Status:                tc.status,
				OperationRVersion:     21,
				OperationTrigger:      codespace_model.OperationTriggerUser,
				OperationCreatedUnix:  time.Now().Unix(),
				OperationStartedUnix:  time.Now().Unix(),
				OperationDeadlineUnix: time.Now().Add(time.Minute).Unix(),
			})
			insertServiceCredentials(t, tc.id)

			result, err := DeleteCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceID: tc.id})
			require.NoError(t, err)
			assert.False(t, result.Deleted)
			assert.Equal(t, codespace_model.StatusDeleting, result.Status)
			assert.Equal(t, codespace_model.OperationDelete, result.OperationType)
			assert.EqualValues(t, 22, result.OperationRVersion)
			row := loadServiceCodespace(t, tc.id)
			assert.Equal(t, codespace_model.StatusDeleting, row.Status)
			assert.Equal(t, codespace_model.OperationDelete, codespace_model.ActiveOperationType(row))
			assert.True(t, codespace_model.IsOperationQueued(row))
			assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_id = ?", tc.id)
			assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_id = ?", tc.id)
		})
	}
}

func TestLifecycleActionValidation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	runningID := int64(1089)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   runningID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    math.MaxInt64,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	_, err := ResumeCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceID: runningID})
	require.ErrorIs(t, err, ErrLifecycleActionStateUnavailable)
	_, err = DeleteCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceID: runningID})
	require.ErrorIs(t, err, ErrLifecycleActionVersionExhausted)
}

func TestLifecycleActionsRequireCreator(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceID := int64(1090)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 24,
	})

	_, err := StopCodespace(t.Context(), LifecycleActionOptions{UserID: 2, CodespaceID: codespaceID})
	require.ErrorIs(t, err, ErrLifecycleActionNotFound)
	row := loadServiceCodespace(t, codespaceID)
	assert.Equal(t, codespace_model.StatusRunning, row.Status)
	assert.False(t, hasActiveOperation(row))
}
