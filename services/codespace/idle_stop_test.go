// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"math"
	"testing"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestIdleStopCreatesAndConfirmsPending(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1043)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     6,
		InteractionGeneration: 3,
	})

	result, err := RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceID:                   codespaceID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 3,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 7, result.GetPending().GetOperationRversion())
	row := loadServiceCodespace(t, codespaceID)
	assert.EqualValues(t, 7, row.OperationRVersion)
	assert.Equal(t, codespace_model.OperationStop, codespace_model.ActiveOperationType(row))
	assert.True(t, codespace_model.IsOperationQueued(row))
	assert.Equal(t, codespace_model.OperationTriggerIdle, row.OperationTrigger)
	assert.Positive(t, row.UpdatedUnix)

	again, err := RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceID:                   codespaceID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 3,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 7, again.GetPending().GetOperationRversion())
}

func TestRequestIdleStopReturnsObservationChanged(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1044)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                     codespaceID,
		Status:                 codespace_model.StatusRunning,
		OperationRVersion:      8,
		InteractionGeneration:  4,
		AutoStopMode:           codespace_model.AutoStopModeCustom,
		AutoStopTimeoutSeconds: 600,
	})

	result, err := RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceID:                   codespaceID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 4,
	})
	require.NoError(t, err)
	settings := result.GetObservationChanged().GetRuntimeSettings()
	assert.True(t, settings.GetAutoStopEnabled())
	assert.EqualValues(t, 600, settings.GetIdleTimeoutSeconds())
	assert.EqualValues(t, 4, settings.GetInteractionGeneration())
	assert.False(t, hasActiveOperation(loadServiceCodespace(t, codespaceID)))
}

func TestRequestIdleStopDisabledReturnsObservationChangedAndKeepsPending(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Codespace.Enabled, false)()

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	runningID := int64(1034)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    runningID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     11,
		InteractionGeneration: 5,
	})

	result, err := RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceID:                   runningID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 5,
	})
	require.NoError(t, err)
	settings := result.GetObservationChanged().GetRuntimeSettings()
	assert.False(t, settings.GetAutoStopEnabled())
	assert.Zero(t, settings.GetIdleTimeoutSeconds())
	assert.EqualValues(t, 5, settings.GetInteractionGeneration())
	row := loadServiceCodespace(t, runningID)
	assert.False(t, hasActiveOperation(row))
	assert.EqualValues(t, 11, row.OperationRVersion)

	pendingID := int64(1036)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   pendingID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    12,
		OperationTrigger:     codespace_model.OperationTriggerIdle,
		OperationCreatedUnix: time.Now().Unix(),
	})
	pending, err := RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceID:                   pendingID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 0,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 12, pending.GetPending().GetOperationRversion())
}

func TestRequestIdleStopNotApplicableAndVersionExhausted(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	conflictID := int64(1045)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   conflictID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    8,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	result, err := RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceID:                   conflictID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 0,
	})
	require.NoError(t, err)
	assert.Equal(t, codespacev1.IdleStopNotApplicableReason_IDLE_STOP_NOT_APPLICABLE_REASON_OPERATION_CONFLICT, result.GetNotApplicable().GetReason())

	stoppedID := int64(1046)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                stoppedID,
		Status:            codespace_model.StatusStopped,
		OperationRVersion: 9,
	})
	result, err = RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceID:                   stoppedID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 0,
	})
	require.NoError(t, err)
	assert.Equal(t, codespacev1.IdleStopNotApplicableReason_IDLE_STOP_NOT_APPLICABLE_REASON_ALREADY_STOPPED, result.GetNotApplicable().GetReason())

	creatingID := int64(1047)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                creatingID,
		Status:            codespace_model.StatusCreating,
		OperationRVersion: 9,
	})
	result, err = RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceID:                   creatingID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 0,
	})
	require.NoError(t, err)
	assert.Equal(t, codespacev1.IdleStopNotApplicableReason_IDLE_STOP_NOT_APPLICABLE_REASON_STATE_UNAVAILABLE, result.GetNotApplicable().GetReason())

	exhaustedID := int64(1048)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                exhaustedID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: math.MaxInt64,
	})
	_, err = RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceID:                   exhaustedID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 0,
	})
	require.ErrorIs(t, err, ErrRequestIdleStopVersionExhausted)
	assert.False(t, hasActiveOperation(loadServiceCodespace(t, exhaustedID)))
}

func TestRequestIdleStopKeepsQueuedCleanupOperation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1035)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     12,
		OperationTrigger:      codespace_model.OperationTriggerIdle,
		OperationCreatedUnix:  time.Now().Add(-time.Hour).Unix(),
		InteractionGeneration: 6,
	})

	fetch, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		CleanupCapacityAvailable: 1,
	})
	require.NoError(t, err)
	require.Len(t, fetch.Operations, 1)
	assert.NotNil(t, fetch.Operations[0].GetStop())
	row := loadServiceCodespace(t, codespaceID)
	require.Equal(t, codespace_model.StatusRunning, row.Status)
	require.Equal(t, codespace_model.OperationStop, codespace_model.ActiveOperationType(row))
	require.True(t, codespace_model.IsOperationRunning(row))
	require.EqualValues(t, 12, row.OperationRVersion)

	assert.Equal(t, codespace_model.OperationTriggerIdle, row.OperationTrigger)
}

func TestRequestIdleStopRejectsManagerMismatch(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	otherManager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	markServiceManagerOnline(t, otherManager, `[{"tag":"default"}]`)
	codespaceID := int64(1049)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 10,
	})

	_, err := RequestIdleStop(t.Context(), otherManager, RequestIdleStopOptions{
		CodespaceID:                   codespaceID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    int64((30 * time.Minute) / time.Second),
		ObservedInteractionGeneration: 0,
	})
	require.ErrorIs(t, err, ErrRequestIdleStopManagerMismatch)
}
