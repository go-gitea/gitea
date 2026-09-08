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
	codespaceUUID := "51515151-5151-4515-8515-515151515151"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     6,
		InteractionGeneration: 3,
	})

	result, err := RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceUUID:                 codespaceUUID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 3,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 7, result.GetPending().GetOperationRversion())
	row := loadServiceCodespace(t, codespaceUUID)
	assert.EqualValues(t, 7, row.OperationRVersion)
	assert.Equal(t, codespace_model.OperationStop, row.OperationType)
	assert.Equal(t, codespace_model.OperationStatusQueued, row.OperationStatus)
	assert.Equal(t, codespace_model.OperationTriggerIdle, row.OperationTrigger)
	assert.Positive(t, row.UpdatedUnix)

	again, err := RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceUUID:                 codespaceUUID,
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
	codespaceUUID := "52525252-5252-4525-8525-525252525252"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                   codespaceUUID,
		Status:                 codespace_model.StatusRunning,
		OperationRVersion:      8,
		InteractionGeneration:  4,
		AutoStopMode:           codespace_model.AutoStopModeCustom,
		AutoStopTimeoutSeconds: 600,
	})

	result, err := RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceUUID:                 codespaceUUID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 4,
	})
	require.NoError(t, err)
	settings := result.GetObservationChanged().GetRuntimeSettings()
	assert.True(t, settings.GetAutoStopEnabled())
	assert.EqualValues(t, 600, settings.GetIdleTimeoutSeconds())
	assert.EqualValues(t, 4, settings.GetInteractionGeneration())
	assert.Empty(t, loadServiceCodespace(t, codespaceUUID).OperationType)
}

func TestRequestIdleStopDisabledReturnsObservationChangedAndKeepsPending(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Codespace.Enabled, false)()

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	runningUUID := "56565656-5656-4565-8565-565656565656"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  runningUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     11,
		InteractionGeneration: 5,
	})

	result, err := RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceUUID:                 runningUUID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 5,
	})
	require.NoError(t, err)
	settings := result.GetObservationChanged().GetRuntimeSettings()
	assert.False(t, settings.GetAutoStopEnabled())
	assert.Zero(t, settings.GetIdleTimeoutSeconds())
	assert.EqualValues(t, 5, settings.GetInteractionGeneration())
	row := loadServiceCodespace(t, runningUUID)
	assert.Empty(t, row.OperationType)
	assert.EqualValues(t, 11, row.OperationRVersion)

	pendingUUID := "57575757-5757-4575-8575-575757575757"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 pendingUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    12,
		OperationType:        codespace_model.OperationStop,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerIdle,
		OperationCreatedUnix: time.Now().Unix(),
	})
	pending, err := RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceUUID:                 pendingUUID,
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
	conflictUUID := "53535353-5353-4535-8535-535353535351"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 conflictUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    8,
		OperationType:        codespace_model.OperationStop,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	result, err := RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceUUID:                 conflictUUID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 0,
	})
	require.NoError(t, err)
	assert.Equal(t, codespacev1.IdleStopNotApplicableReason_IDLE_STOP_NOT_APPLICABLE_REASON_OPERATION_CONFLICT, result.GetNotApplicable().GetReason())

	stoppedUUID := "53535353-5353-4535-8535-535353535353"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              stoppedUUID,
		Status:            codespace_model.StatusStopped,
		OperationRVersion: 9,
	})
	result, err = RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceUUID:                 stoppedUUID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 0,
	})
	require.NoError(t, err)
	assert.Equal(t, codespacev1.IdleStopNotApplicableReason_IDLE_STOP_NOT_APPLICABLE_REASON_ALREADY_STOPPED, result.GetNotApplicable().GetReason())

	creatingUUID := "53535353-5353-4535-8535-535353535354"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              creatingUUID,
		Status:            codespace_model.StatusCreating,
		OperationRVersion: 9,
	})
	result, err = RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceUUID:                 creatingUUID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 0,
	})
	require.NoError(t, err)
	assert.Equal(t, codespacev1.IdleStopNotApplicableReason_IDLE_STOP_NOT_APPLICABLE_REASON_STATE_UNAVAILABLE, result.GetNotApplicable().GetReason())

	exhaustedUUID := "54545454-5454-4545-8545-545454545454"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              exhaustedUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: math.MaxInt64,
	})
	_, err = RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceUUID:                 exhaustedUUID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    1800,
		ObservedInteractionGeneration: 0,
	})
	require.ErrorIs(t, err, ErrRequestIdleStopVersionExhausted)
	assert.Empty(t, loadServiceCodespace(t, exhaustedUUID).OperationType)
}

func TestRequestIdleStopCreatesNewVersionAfterQueuedIdleTimeout(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "56565656-5656-4565-8565-565656565657"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     12,
		OperationType:         codespace_model.OperationStop,
		OperationStatus:       codespace_model.OperationStatusQueued,
		OperationTrigger:      codespace_model.OperationTriggerIdle,
		OperationCreatedUnix:  time.Now().Add(-setting.Codespace.QueueTimeout - time.Second).Unix(),
		InteractionGeneration: 6,
	})

	fetch, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		CleanupCapacityAvailable: 1,
	})
	require.NoError(t, err)
	assert.Empty(t, fetch.Operations)
	row := loadServiceCodespace(t, codespaceUUID)
	require.Equal(t, codespace_model.StatusRunning, row.Status)
	require.Empty(t, row.OperationType)
	require.EqualValues(t, 12, row.OperationRVersion)

	result, err := RequestIdleStop(t.Context(), manager, RequestIdleStopOptions{
		CodespaceUUID:                 codespaceUUID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    int64(setting.Codespace.AutoStopDefaultTimeout / time.Second),
		ObservedInteractionGeneration: 6,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 13, result.GetPending().GetOperationRversion())
	row = loadServiceCodespace(t, codespaceUUID)
	assert.Equal(t, codespace_model.OperationStop, row.OperationType)
	assert.Equal(t, codespace_model.OperationStatusQueued, row.OperationStatus)
	assert.Equal(t, codespace_model.OperationTriggerIdle, row.OperationTrigger)
}

func TestRequestIdleStopRejectsManagerMismatch(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	otherManager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	markServiceManagerOnline(t, otherManager, `[{"tag":"default"}]`)
	codespaceUUID := "55555555-5555-4555-8555-555555555555"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 10,
	})

	_, err := RequestIdleStop(t.Context(), otherManager, RequestIdleStopOptions{
		CodespaceUUID:                 codespaceUUID,
		ObservedAutoStopEnabled:       true,
		ObservedIdleTimeoutSeconds:    int64((30 * time.Minute) / time.Second),
		ObservedInteractionGeneration: 0,
	})
	require.ErrorIs(t, err, ErrRequestIdleStopManagerMismatch)
}
