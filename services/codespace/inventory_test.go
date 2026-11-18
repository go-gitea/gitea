// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"strings"
	"testing"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/contexts"
)

func TestReportInstancesReturnsSettingsAndActions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	otherManager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	markServiceManagerOnline(t, otherManager, `[{"tag":"default"}]`)
	runningID := int64(1053)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    runningID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     81,
		InteractionGeneration: 6,
	})
	activeID := int64(1054)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   activeID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    82,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	stoppedID := int64(1055)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                stoppedID,
		Status:            codespace_model.StatusStopped,
		OperationRVersion: 83,
	})
	failedID := int64(1056)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                failedID,
		Status:            codespace_model.StatusFailed,
		OperationRVersion: 84,
	})
	otherID := int64(1057)
	insertServiceCodespace(t, otherManager.ID, &codespace_model.Codespace{
		ID:                otherID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 85,
	})
	unassignedID := int64(1058)
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		ID:                   unassignedID,
		Status:               codespace_model.StatusCreating,
		OperationRVersion:    86,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	absentID := int64(1059)

	_, err := ReportInstances(t.Context(), manager, ReportInstancesOptions{
		InventoryGeneration: 1,
		Instances: []*codespacev1.RuntimeInstanceRef{
			{CodespaceId: runningID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING},
			{CodespaceId: activeID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING, ObservedOperationRversion: 81},
			{CodespaceId: runningID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_STOPPED},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")

	result, err := ReportInstances(t.Context(), manager, ReportInstancesOptions{
		InventoryGeneration: 1,
		Instances: []*codespacev1.RuntimeInstanceRef{
			{CodespaceId: runningID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING},
			{CodespaceId: activeID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING, ObservedOperationRversion: 81},
			{CodespaceId: stoppedID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING},
			{CodespaceId: failedID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_STOPPED},
			{CodespaceId: otherID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING},
			{CodespaceId: unassignedID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_CREATING},
			{CodespaceId: absentID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_STOPPED},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Results, 7)
	assert.NotNil(t, result.Results[0].RuntimeSettings)
	assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_UNSPECIFIED, result.Results[0].GetAction())
	assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_REFETCH_OPERATION, result.Results[1].GetAction())
	assert.EqualValues(t, 82, result.Results[1].GetCurrentOperationRversion())
	assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_STOP_LOCAL_RUNTIME, result.Results[2].GetAction())
	assert.EqualValues(t, 83, result.Results[2].GetCurrentOperationRversion())
	assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEANUP_LOCAL_RUNTIME, result.Results[3].GetAction())
	assert.Nil(t, result.Results[3].RuntimeSettings)
	assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEANUP_LOCAL_RUNTIME, result.Results[4].GetAction())
	assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEANUP_LOCAL_RUNTIME, result.Results[5].GetAction())
	assert.Nil(t, result.Results[5].RuntimeSettings)
	assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEANUP_LOCAL_RUNTIME, result.Results[6].GetAction())
	assert.EqualValues(t, 1, loadServiceManager(t, manager.ID).InventoryGeneration)
}

func TestReportInstancesReturnsDisabledRuntimeSettings(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Codespace.Enabled, false)()

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1060)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     87,
		InteractionGeneration: 9,
	})

	result, err := ReportInstances(t.Context(), manager, ReportInstancesOptions{
		InventoryGeneration: 11,
		Instances: []*codespacev1.RuntimeInstanceRef{{
			CodespaceId:  codespaceID,
			RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING,
		}},
	})
	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.NotNil(t, result.Results[0].RuntimeSettings)
	assert.Zero(t, result.Results[0].RuntimeSettings.GetIdleTimeoutSeconds())
	assert.EqualValues(t, 9, result.Results[0].RuntimeSettings.GetInteractionGeneration())
	assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_UNSPECIFIED, result.Results[0].GetAction())
}

func TestReportInstancesAppliesStoppedFactAndClearsOperationContext(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	stoppedCodespaceID := int64(1061)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                stoppedCodespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 91,
	})
	clearID := int64(1062)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                clearID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 93,
	})

	result, err := ReportInstances(t.Context(), manager, ReportInstancesOptions{
		InventoryGeneration: 2,
		Instances: []*codespacev1.RuntimeInstanceRef{
			{CodespaceId: stoppedCodespaceID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_STOPPED},
			{CodespaceId: clearID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING, ObservedOperationRversion: 93},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Results, 2)
	assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_UNSPECIFIED, result.Results[0].GetAction())
	assert.Equal(t, codespace_model.StatusStopped, loadServiceCodespace(t, stoppedCodespaceID).Status)
	assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEAR_OPERATION_CONTEXT, result.Results[1].GetAction())
	assert.EqualValues(t, 93, result.Results[1].GetCurrentOperationRversion())
}

func TestReportInstancesGenerationAndMissingState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	missingRunningID := int64(1063)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                missingRunningID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 101,
	})
	missingCreateID := int64(1064)
	missingCreate := &codespace_model.Codespace{
		ID:                    missingCreateID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     102,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Unix(),
		OperationStartedUnix:  time.Now().Unix(),
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	}
	insertServiceCodespace(t, manager.ID, missingCreate)

	result, err := ReportInstances(t.Context(), manager, ReportInstancesOptions{
		InventoryGeneration: 3,
	})
	require.NoError(t, err)
	assert.Empty(t, result.Results)
	missingRunning := loadServiceCodespace(t, missingRunningID)
	assert.Equal(t, codespace_model.StatusFailed, missingRunning.Status)
	assert.Zero(t, missingRunning.RuntimeOperationRVersion)
	require.NoError(t, codespace_model.ValidateCodespace(missingRunning))
	assert.Contains(t, readServiceLog(t, codespaceLogFilename(missingRunning.ID)), "Gitea recorded missing runtime as failed.")
	assert.Equal(t, codespace_model.StatusCreating, loadServiceCodespace(t, missingCreateID).Status)

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
	assert.EqualValues(t, 3, loadServiceManager(t, manager.ID).InventoryGeneration)
}

type inventoryQueryCounter struct{ reads int }

func (c *inventoryQueryCounter) BeforeProcess(h *contexts.ContextHook) (context.Context, error) {
	if h.Ctx.Value(c) == true && strings.HasPrefix(h.SQL, "SELECT") {
		c.reads++
	}
	return h.Ctx, nil
}

func (*inventoryQueryCounter) AfterProcess(*contexts.ContextHook) error { return nil }

func TestReportInstancesBatchesUnchangedInventory(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	manager := insertServiceManager(t)
	instances := make([]*codespacev1.RuntimeInstanceRef, 0, db.DefaultMaxInSize+1)
	for i := range db.DefaultMaxInSize + 1 {
		id := int64(2000 + i)
		insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
			ID: id, Status: codespace_model.StatusRunning, OperationRVersion: 1,
		})
		instances = append(instances, &codespacev1.RuntimeInstanceRef{
			CodespaceId: id, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING, ObservedOperationRversion: 1,
		})
	}
	counter := new(inventoryQueryCounter)
	unittest.GetXORMEngine().AddHook(counter)
	ctx := context.WithValue(t.Context(), counter, true)
	result, err := ReportInstances(ctx, manager, ReportInstancesOptions{InventoryGeneration: 1, Instances: instances})
	require.NoError(t, err)
	require.Len(t, result.Results, len(instances))
	for i, row := range result.Results {
		assert.Equal(t, instances[i].CodespaceId, row.CodespaceId)
		assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEAR_OPERATION_CONTEXT, row.Action)
	}
	batches := (len(instances) + db.DefaultMaxInSize - 1) / db.DefaultMaxInSize
	assert.LessOrEqual(t, counter.reads, 2*batches+3)
	t.Logf("%d instances reconciled with %d SELECTs", len(instances), counter.reads)
}

func TestProcessMissingRuntimeInstanceRechecksGenerationAndBinding(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	otherManager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	markServiceManagerOnline(t, otherManager, `[{"tag":"default"}]`)
	staleID := int64(1065)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                staleID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 103,
	})
	currentManager := loadServiceManager(t, manager.ID)
	currentManager.InventoryGeneration = 9
	_, err := db.GetEngine(t.Context()).ID(manager.ID).Cols("inventory_generation").Update(currentManager)
	require.NoError(t, err)

	err = processMissingRuntimeInstance(t.Context(), manager.ID, 8, staleID, time.Now().Unix())
	var stale *StaleGenerationError
	require.ErrorAs(t, err, &stale)
	assert.EqualValues(t, 9, stale.CurrentGeneration)
	assert.Equal(t, codespace_model.StatusRunning, loadServiceCodespace(t, staleID).Status)

	movedID := int64(1066)
	insertServiceCodespace(t, otherManager.ID, &codespace_model.Codespace{
		ID:                movedID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 104,
	})

	require.NoError(t, processMissingRuntimeInstance(t.Context(), manager.ID, 9, movedID, time.Now().Unix()))
	assert.Equal(t, codespace_model.StatusRunning, loadServiceCodespace(t, movedID).Status)
	assert.Equal(t, otherManager.ID, loadServiceCodespace(t, movedID).ManagerID)
}

func TestReportInstancesRejectsStateHistoryConflict(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1067)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 111,
	})

	_, err := ReportInstances(t.Context(), manager, ReportInstancesOptions{
		InventoryGeneration: 4,
		Instances: []*codespacev1.RuntimeInstanceRef{{
			CodespaceId:               codespaceID,
			RuntimeState:              codespacev1.RuntimeState_RUNTIME_STATE_RUNNING,
			ObservedOperationRversion: 112,
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
