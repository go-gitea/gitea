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
	codespaceUUID := "56565656-5656-4565-8565-565656565656"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     11,
		OperationType:         codespace_model.OperationStop,
		OperationStatus:       codespace_model.OperationStatusQueued,
		OperationTrigger:      codespace_model.OperationTriggerIdle,
		OperationCreatedUnix:  time.Now().Unix(),
		InteractionGeneration: 4,
	})

	result, err := ContinueCodespace(t.Context(), ContinueCodespaceOptions{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 5, result.InteractionGeneration)
	row := loadServiceCodespace(t, codespaceUUID)
	assert.EqualValues(t, 5, row.InteractionGeneration)
	assert.Empty(t, row.OperationType)
	assert.Empty(t, row.OperationStatus)
	assert.Empty(t, row.OperationTrigger)
	assert.Positive(t, row.LastActiveUnix)
}

func TestContinueCodespaceKeepsLifecycleUpdatedUnixWithoutIdleStop(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceUUID := "56565656-5656-4565-8565-565656565657"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     11,
		InteractionGeneration: 4,
	})

	result, err := ContinueCodespace(t.Context(), ContinueCodespaceOptions{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 5, result.InteractionGeneration)
	row := loadServiceCodespace(t, codespaceUUID)
	assert.EqualValues(t, 1, row.UpdatedUnix)
	assert.Positive(t, row.LastActiveUnix)
}

func TestContinueCodespaceRejectsRunningStopAndVersionExhausted(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	runningStopUUID := "57575757-5757-4575-8575-575757575757"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  runningStopUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     12,
		OperationType:         codespace_model.OperationStop,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerIdle,
		OperationCreatedUnix:  time.Now().Unix(),
		OperationStartedUnix:  time.Now().Unix(),
		OperationDeadlineUnix: time.Now().Add(time.Minute).Unix(),
	})
	_, err := ContinueCodespace(t.Context(), ContinueCodespaceOptions{
		UserID:        1,
		CodespaceUUID: runningStopUUID,
	})
	require.ErrorIs(t, err, ErrInteractionStateUnavailable)

	exhaustedUUID := "58585858-5858-4585-8585-585858585858"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  exhaustedUUID,
		Status:                codespace_model.StatusRunning,
		InteractionGeneration: math.MaxInt64,
	})
	_, err = ContinueCodespace(t.Context(), ContinueCodespaceOptions{
		UserID:        1,
		CodespaceUUID: exhaustedUUID,
	})
	require.ErrorIs(t, err, ErrInteractionVersionExhausted)
}

func TestUpdateAutoStopCancelsQueuedIdleOnlyWhenRuntimePolicyChanges(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	changedUUID := "59595959-5959-4595-8595-595959595959"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 changedUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    13,
		OperationType:        codespace_model.OperationStop,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerIdle,
		OperationCreatedUnix: time.Now().Unix(),
	})
	result, err := UpdateAutoStop(t.Context(), UpdateAutoStopOptions{
		UserID:               1,
		CodespaceUUID:        changedUUID,
		Mode:                 codespace_model.AutoStopModeCustom,
		CustomTimeoutSeconds: int64((10 * time.Minute) / time.Second),
	})
	require.NoError(t, err)
	assert.Equal(t, codespace_model.AutoStopModeCustom, result.Mode)
	assert.EqualValues(t, 600, result.CustomTimeoutSeconds)
	assert.EqualValues(t, 600, result.RuntimeSettings.IdleTimeoutSeconds)
	row := loadServiceCodespace(t, changedUUID)
	assert.Empty(t, row.OperationType)
	assert.Empty(t, row.OperationStatus)
	assert.Greater(t, row.UpdatedUnix, int64(1))

	samePolicyUUID := "60606060-6060-4606-8606-606060606060"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 samePolicyUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    14,
		OperationType:        codespace_model.OperationStop,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerIdle,
		OperationCreatedUnix: time.Now().Unix(),
	})
	_, err = UpdateAutoStop(t.Context(), UpdateAutoStopOptions{
		UserID:               1,
		CodespaceUUID:        samePolicyUUID,
		Mode:                 codespace_model.AutoStopModeCustom,
		CustomTimeoutSeconds: int64(setting.Codespace.AutoStopDefaultTimeout / time.Second),
	})
	require.NoError(t, err)
	row = loadServiceCodespace(t, samePolicyUUID)
	assert.Equal(t, codespace_model.OperationStop, row.OperationType)
	assert.Equal(t, codespace_model.OperationStatusQueued, row.OperationStatus)
	assert.Equal(t, codespace_model.OperationTriggerIdle, row.OperationTrigger)
}

func TestUpdateAutoStopKeepsLifecycleUpdatedUnixWithoutIdleStopCancellation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceUUID := "60606060-6060-4606-8606-606060606061"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:   codespaceUUID,
		Status: codespace_model.StatusRunning,
	})

	_, err := UpdateAutoStop(t.Context(), UpdateAutoStopOptions{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		Mode:          codespace_model.AutoStopModeNever,
	})
	require.NoError(t, err)
	row := loadServiceCodespace(t, codespaceUUID)
	assert.Equal(t, codespace_model.AutoStopModeNever, row.AutoStopMode)
	assert.EqualValues(t, 1, row.UpdatedUnix)
}

func TestUpdateAutoStopValidationAndState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	stoppedUUID := "61616161-6161-4616-8616-616161616161"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:   stoppedUUID,
		Status: codespace_model.StatusStopped,
	})
	_, err := UpdateAutoStop(t.Context(), UpdateAutoStopOptions{
		UserID:        1,
		CodespaceUUID: stoppedUUID,
		Mode:          codespace_model.AutoStopModeNever,
	})
	require.NoError(t, err)
	row := loadServiceCodespace(t, stoppedUUID)
	assert.Equal(t, codespace_model.AutoStopModeNever, row.AutoStopMode)
	assert.Zero(t, row.AutoStopTimeoutSeconds)

	_, err = UpdateAutoStop(t.Context(), UpdateAutoStopOptions{
		UserID:               1,
		CodespaceUUID:        stoppedUUID,
		Mode:                 codespace_model.AutoStopModeCustom,
		CustomTimeoutSeconds: int64((setting.Codespace.AutoStopMinTimeout / time.Second) - 1),
	})
	require.Error(t, err)

	creatingUUID := "62626262-6262-4626-8626-626262626262"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:   creatingUUID,
		Status: codespace_model.StatusCreating,
	})
	_, err = UpdateAutoStop(t.Context(), UpdateAutoStopOptions{
		UserID:        1,
		CodespaceUUID: creatingUUID,
		Mode:          codespace_model.AutoStopModeNever,
	})
	require.ErrorIs(t, err, ErrInteractionStateUnavailable)
}
