// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"math"
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContinueCodespaceCancelsQueuedIdleStop(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceID := int64(1034)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     11,
		OperationTrigger:      codespace_model.OperationTriggerIdle,
		OperationCreatedUnix:  time.Now().Unix(),
		InteractionGeneration: 4,
	})

	result, err := ContinueCodespace(t.Context(), ContinueCodespaceOptions{
		CodespaceID: codespaceID,
		UserID:      1,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 5, result.InteractionGeneration)
	row := loadServiceCodespace(t, codespaceID)
	assert.EqualValues(t, 5, row.InteractionGeneration)
	assert.False(t, hasActiveOperation(row))
	assert.Empty(t, row.OperationTrigger)
	assert.Positive(t, row.LastActiveUnix)
}

func TestContinueCodespaceKeepsLifecycleUpdatedUnixWithoutIdleStop(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceID := int64(1035)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     11,
		InteractionGeneration: 4,
	})

	result, err := ContinueCodespace(t.Context(), ContinueCodespaceOptions{
		CodespaceID: codespaceID,
		UserID:      1,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 5, result.InteractionGeneration)
	row := loadServiceCodespace(t, codespaceID)
	assert.EqualValues(t, 1, row.UpdatedUnix)
	assert.Positive(t, row.LastActiveUnix)
}

func TestContinueCodespaceRejectsRunningStopAndVersionExhausted(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	runningStopID := int64(1036)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    runningStopID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     12,
		OperationTrigger:      codespace_model.OperationTriggerIdle,
		OperationCreatedUnix:  time.Now().Unix(),
		OperationStartedUnix:  time.Now().Unix(),
		OperationDeadlineUnix: time.Now().Add(time.Minute).Unix(),
	})
	_, err := ContinueCodespace(t.Context(), ContinueCodespaceOptions{
		CodespaceID: runningStopID,
		UserID:      1,
	})
	require.ErrorIs(t, err, ErrInteractionStateUnavailable)

	exhaustedID := int64(1037)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    exhaustedID,
		Status:                codespace_model.StatusRunning,
		InteractionGeneration: math.MaxInt64,
	})
	_, err = ContinueCodespace(t.Context(), ContinueCodespaceOptions{
		CodespaceID: exhaustedID,
		UserID:      1,
	})
	require.ErrorIs(t, err, ErrInteractionVersionExhausted)
}

func TestUpdateAutoStopCancelsQueuedIdleOnlyWhenRuntimePolicyChanges(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	changedID := int64(1038)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   changedID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    13,
		OperationTrigger:     codespace_model.OperationTriggerIdle,
		OperationCreatedUnix: time.Now().Unix(),
	})
	result, err := UpdateAutoStop(t.Context(), UpdateAutoStopOptions{
		CodespaceID:          changedID,
		UserID:               1,
		Mode:                 codespace_model.AutoStopModeCustom,
		CustomTimeoutSeconds: int64((10 * time.Minute) / time.Second),
	})
	require.NoError(t, err)
	assert.Equal(t, codespace_model.AutoStopModeCustom, result.Mode)
	assert.EqualValues(t, 600, result.CustomTimeoutSeconds)
	assert.EqualValues(t, 600, result.RuntimeSettings.IdleTimeoutSeconds)
	row := loadServiceCodespace(t, changedID)
	assert.False(t, hasActiveOperation(row))
	assert.Greater(t, row.UpdatedUnix, int64(1))

	samePolicyID := int64(1039)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   samePolicyID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    14,
		OperationTrigger:     codespace_model.OperationTriggerIdle,
		OperationCreatedUnix: time.Now().Unix(),
	})
	_, err = UpdateAutoStop(t.Context(), UpdateAutoStopOptions{
		CodespaceID:          samePolicyID,
		UserID:               1,
		Mode:                 codespace_model.AutoStopModeCustom,
		CustomTimeoutSeconds: int64(setting.Codespace.AutoStopDefaultTimeout / time.Second),
	})
	require.NoError(t, err)
	row = loadServiceCodespace(t, samePolicyID)
	assert.Equal(t, codespace_model.OperationStop, codespace_model.ActiveOperationType(row))
	assert.True(t, codespace_model.IsOperationQueued(row))
	assert.Equal(t, codespace_model.OperationTriggerIdle, row.OperationTrigger)
}

func TestUpdateAutoStopKeepsLifecycleUpdatedUnixWithoutIdleStopCancellation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceID := int64(1040)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:     codespaceID,
		Status: codespace_model.StatusRunning,
	})

	_, err := UpdateAutoStop(t.Context(), UpdateAutoStopOptions{
		CodespaceID: codespaceID,
		UserID:      1,
		Mode:        codespace_model.AutoStopModeNever,
	})
	require.NoError(t, err)
	row := loadServiceCodespace(t, codespaceID)
	assert.Equal(t, codespace_model.AutoStopModeNever, row.AutoStopMode)
	assert.EqualValues(t, 1, row.UpdatedUnix)
}

func TestUpdateAutoStopValidationAndState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	stoppedID := int64(1041)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:     stoppedID,
		Status: codespace_model.StatusStopped,
	})
	_, err := UpdateAutoStop(t.Context(), UpdateAutoStopOptions{
		CodespaceID: stoppedID,
		UserID:      1,
		Mode:        codespace_model.AutoStopModeNever,
	})
	require.NoError(t, err)
	row := loadServiceCodespace(t, stoppedID)
	assert.Equal(t, codespace_model.AutoStopModeNever, row.AutoStopMode)
	assert.Zero(t, row.AutoStopTimeoutSeconds)

	_, err = UpdateAutoStop(t.Context(), UpdateAutoStopOptions{
		CodespaceID:          stoppedID,
		UserID:               1,
		Mode:                 codespace_model.AutoStopModeCustom,
		CustomTimeoutSeconds: int64((setting.Codespace.AutoStopMinTimeout / time.Second) - 1),
	})
	require.Error(t, err)

	creatingID := int64(1042)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:     creatingID,
		Status: codespace_model.StatusCreating,
	})
	_, err = UpdateAutoStop(t.Context(), UpdateAutoStopOptions{
		CodespaceID: creatingID,
		UserID:      1,
		Mode:        codespace_model.AutoStopModeNever,
	})
	require.ErrorIs(t, err, ErrInteractionStateUnavailable)
}
