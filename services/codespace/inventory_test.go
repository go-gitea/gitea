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
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportInstancesReturnsSettingsAndActions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	otherManager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	markServiceManagerOnline(t, otherManager, `["default"]`)
	runningUUID := "a1a1a1a1-a1a1-4a1a-8a1a-a1a1a1a1a1a1"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  runningUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     81,
		InteractionGeneration: 6,
	})
	activeUUID := "a2a2a2a2-a2a2-4a2a-8a2a-a2a2a2a2a2a2"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 activeUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    82,
		OperationType:        codespace_model.OperationStop,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	stoppedUUID := "a3a3a3a3-a3a3-4a3a-8a3a-a3a3a3a3a3a3"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              stoppedUUID,
		Status:            codespace_model.StatusStopped,
		OperationRVersion: 83,
	})
	failedUUID := "a4a4a4a4-a4a4-4a4a-8a4a-a4a4a4a4a4a4"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              failedUUID,
		Status:            codespace_model.StatusFailed,
		OperationRVersion: 84,
	})
	otherUUID := "a5a5a5a5-a5a5-4a5a-8a5a-a5a5a5a5a5a5"
	insertServiceCodespace(t, otherManager.ID, &codespace_model.Codespace{
		UUID:              otherUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 85,
	})
	unboundUUID := "a6a6a6a6-a6a6-4a6a-8a6a-a6a6a6a6a6a6"
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		UUID:                 unboundUUID,
		Status:               codespace_model.StatusCreating,
		OperationRVersion:    86,
		OperationType:        codespace_model.OperationCreate,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	absentUUID := "a7a7a7a7-a7a7-4a7a-8a7a-a7a7a7a7a7a7"

	_, err := ReportInstances(t.Context(), manager, ReportInstancesOptions{
		InventoryGeneration: 1,
		Instances: []RuntimeInstanceRef{
			{CodespaceUUID: runningUUID, RuntimeState: RuntimeInstanceStateRunning},
			{CodespaceUUID: activeUUID, RuntimeState: RuntimeInstanceStateRunning, ObservedOperationRVersion: 81},
			{CodespaceUUID: runningUUID, RuntimeState: RuntimeInstanceStateStopped},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")

	result, err := ReportInstances(t.Context(), manager, ReportInstancesOptions{
		InventoryGeneration: 1,
		Instances: []RuntimeInstanceRef{
			{CodespaceUUID: runningUUID, RuntimeState: RuntimeInstanceStateRunning},
			{CodespaceUUID: activeUUID, RuntimeState: RuntimeInstanceStateRunning, ObservedOperationRVersion: 81},
			{CodespaceUUID: stoppedUUID, RuntimeState: RuntimeInstanceStateRunning},
			{CodespaceUUID: failedUUID, RuntimeState: RuntimeInstanceStateFailed},
			{CodespaceUUID: otherUUID, RuntimeState: RuntimeInstanceStateRunning},
			{CodespaceUUID: unboundUUID, RuntimeState: RuntimeInstanceStateCreating},
			{CodespaceUUID: absentUUID, RuntimeState: RuntimeInstanceStateFailed},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Results, 7)
	assert.NotNil(t, result.Results[0].RuntimeSettings)
	assert.Empty(t, result.Results[0].Action)
	assert.Equal(t, InventoryActionRefetchOperation, result.Results[1].Action)
	assert.EqualValues(t, 82, result.Results[1].CurrentOperationRVersion)
	assert.Equal(t, InventoryActionStopLocalRuntime, result.Results[2].Action)
	assert.EqualValues(t, 83, result.Results[2].CurrentOperationRVersion)
	assert.Equal(t, InventoryActionCleanupLocalRuntime, result.Results[3].Action)
	assert.Nil(t, result.Results[3].RuntimeSettings)
	assert.Equal(t, InventoryActionCleanupLocalRuntime, result.Results[4].Action)
	assert.Empty(t, result.Results[5].Action)
	assert.Nil(t, result.Results[5].RuntimeSettings)
	assert.Equal(t, InventoryActionCleanupLocalRuntime, result.Results[6].Action)
	assert.EqualValues(t, 1, loadServiceManager(t, manager.ID).InventoryGeneration)
}

func TestReportInstancesReturnsDisabledRuntimeSettings(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Codespace.Enabled, false)()

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "a8a8a8a8-a8a8-4a8a-8a8a-a8a8a8a8a8a8"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     87,
		InteractionGeneration: 9,
	})

	result, err := ReportInstances(t.Context(), manager, ReportInstancesOptions{
		InventoryGeneration: 11,
		Instances: []RuntimeInstanceRef{{
			CodespaceUUID: codespaceUUID,
			RuntimeState:  RuntimeInstanceStateRunning,
		}},
	})
	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.NotNil(t, result.Results[0].RuntimeSettings)
	assert.False(t, result.Results[0].RuntimeSettings.AutoStopEnabled)
	assert.Zero(t, result.Results[0].RuntimeSettings.IdleTimeoutSeconds)
	assert.EqualValues(t, 9, result.Results[0].RuntimeSettings.InteractionGeneration)
	assert.Empty(t, result.Results[0].Action)
}

func TestReportInstancesTransitionAndClearActions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	stoppedRuntimeUUID := "b1b1b1b1-b1b1-4b1b-8b1b-b1b1b1b1b1b1"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              stoppedRuntimeUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 91,
	})
	failedRuntimeUUID := "b2b2b2b2-b2b2-4b2b-8b2b-b2b2b2b2b2b2"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              failedRuntimeUUID,
		Status:            codespace_model.StatusStopped,
		OperationRVersion: 92,
	})
	clearUUID := "b3b3b3b3-b3b3-4b3b-8b3b-b3b3b3b3b3b3"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              clearUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 93,
	})

	result, err := ReportInstances(t.Context(), manager, ReportInstancesOptions{
		InventoryGeneration: 2,
		Instances: []RuntimeInstanceRef{
			{CodespaceUUID: stoppedRuntimeUUID, RuntimeState: RuntimeInstanceStateStopped},
			{CodespaceUUID: failedRuntimeUUID, RuntimeState: RuntimeInstanceStateFailed},
			{CodespaceUUID: clearUUID, RuntimeState: RuntimeInstanceStateRunning, ObservedOperationRVersion: 93},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Results, 3)
	assert.Equal(t, InventoryActionReportRuntimeTransition, result.Results[0].Action)
	assert.EqualValues(t, 91, result.Results[0].CurrentOperationRVersion)
	assert.Equal(t, InventoryActionReportRuntimeTransition, result.Results[1].Action)
	assert.EqualValues(t, 92, result.Results[1].CurrentOperationRVersion)
	assert.Equal(t, InventoryActionClearOperationContext, result.Results[2].Action)
	assert.EqualValues(t, 93, result.Results[2].CurrentOperationRVersion)
}

func TestReportInstancesGenerationAndMissingState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	missingRunningUUID := "c1c1c1c1-c1c1-4c1c-8c1c-c1c1c1c1c1c1"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              missingRunningUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 101,
	})
	missingCreateUUID := "c2c2c2c2-c2c2-4c2c-8c2c-c2c2c2c2c2c2"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  missingCreateUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     102,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Unix(),
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})

	result, err := ReportInstances(t.Context(), manager, ReportInstancesOptions{
		InventoryGeneration: 3,
	})
	require.NoError(t, err)
	assert.Empty(t, result.Results)
	missingRunning := loadServiceCodespace(t, missingRunningUUID)
	assert.Equal(t, codespace_model.StatusFailed, missingRunning.Status)
	assert.Contains(t, readServiceLog(t, missingRunning.LogFilename), "Gitea recorded missing runtime as failed.")
	assert.Equal(t, codespace_model.StatusCreating, loadServiceCodespace(t, missingCreateUUID).Status)

	result, err = ReportInstances(t.Context(), manager, ReportInstancesOptions{
		InventoryGeneration: 3,
	})
	require.NoError(t, err)
	assert.Empty(t, result.Results)

	_, err = ReportInstances(t.Context(), manager, ReportInstancesOptions{
		InventoryGeneration: 2,
	})
	var stale *StaleGenerationError
	require.ErrorAs(t, err, &stale)
	assert.EqualValues(t, 3, stale.CurrentGeneration)
}

func TestAcceptInventoryGenerationDoesNotRegress(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	require.NoError(t, acceptInventoryGeneration(t.Context(), manager.ID, 9))
	err := acceptInventoryGeneration(t.Context(), manager.ID, 8)
	var stale *StaleGenerationError
	require.ErrorAs(t, err, &stale)
	assert.EqualValues(t, 9, stale.CurrentGeneration)
	assert.EqualValues(t, 9, loadServiceManager(t, manager.ID).InventoryGeneration)
}

func TestProcessMissingRuntimeInstanceRechecksGenerationAndBinding(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	otherManager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	markServiceManagerOnline(t, otherManager, `["default"]`)
	staleUUID := "c3c3c3c3-c3c3-4c3c-8c3c-c3c3c3c3c3c3"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              staleUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 103,
	})
	currentManager := loadServiceManager(t, manager.ID)
	currentManager.InventoryGeneration = 9
	_, err := db.GetEngine(t.Context()).ID(manager.ID).Cols("inventory_generation").Update(currentManager)
	require.NoError(t, err)

	err = processMissingRuntimeInstance(t.Context(), manager.ID, 8, staleUUID, time.Now().Unix())
	var stale *StaleGenerationError
	require.ErrorAs(t, err, &stale)
	assert.EqualValues(t, 9, stale.CurrentGeneration)
	assert.Equal(t, codespace_model.StatusRunning, loadServiceCodespace(t, staleUUID).Status)

	movedUUID := "c4c4c4c4-c4c4-4c4c-8c4c-c4c4c4c4c4c4"
	insertServiceCodespace(t, otherManager.ID, &codespace_model.Codespace{
		UUID:              movedUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 104,
	})

	require.NoError(t, processMissingRuntimeInstance(t.Context(), manager.ID, 9, movedUUID, time.Now().Unix()))
	assert.Equal(t, codespace_model.StatusRunning, loadServiceCodespace(t, movedUUID).Status)
	assert.Equal(t, otherManager.ID, loadServiceCodespace(t, movedUUID).ManagerID)
}

func TestReportInstancesRejectsStateHistoryConflict(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "d1d1d1d1-d1d1-4d1d-8d1d-d1d1d1d1d1d1"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 111,
	})

	_, err := ReportInstances(t.Context(), manager, ReportInstancesOptions{
		InventoryGeneration: 4,
		Instances: []RuntimeInstanceRef{{
			CodespaceUUID:             codespaceUUID,
			RuntimeState:              RuntimeInstanceStateRunning,
			ObservedOperationRVersion: 112,
		}},
	})
	require.ErrorIs(t, err, ErrReportInstancesStateHistoryConflict)
	assert.EqualValues(t, 0, loadServiceManager(t, manager.ID).InventoryGeneration)
}

func loadServiceManager(t *testing.T, managerID int64) *codespace_model.Manager {
	t.Helper()
	manager := new(codespace_model.Manager)
	has, err := db.GetEngine(t.Context()).ID(managerID).Get(manager)
	require.NoError(t, err)
	require.True(t, has)
	return manager
}
