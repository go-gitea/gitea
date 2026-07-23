// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportRuntimeTransitionStoppedFromRunning(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "91919191-9191-4919-8919-919191919191"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 71,
		RuntimeGeneration: 3,
	})
	insertServiceCredentials(t, codespaceUUID)
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 71, []map[string]any{})))

	err := ReportRuntimeTransition(t.Context(), manager, ReportRuntimeTransitionOptions{
		CodespaceUUID:             codespaceUUID,
		RuntimeGeneration:         4,
		ObservedOperationRVersion: 71,
		RuntimeState:              RuntimeTransitionStopped,
	})
	require.NoError(t, err)

	row := loadServiceCodespace(t, codespaceUUID)
	assert.Equal(t, codespace_model.StatusStopped, row.Status)
	assert.EqualValues(t, 4, row.RuntimeGeneration)
	assert.Positive(t, row.StoppedUnix)
	assert.Positive(t, row.UpdatedUnix)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)
	assertServiceExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", codespaceUUID)
	hasReady, err := HasReadyRuntimeMetadata(t.Context(), codespaceUUID, 71)
	require.NoError(t, err)
	assert.False(t, hasReady)
}

func TestReportRuntimeTransitionFailedFromStoppedAndIdempotent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "92929292-9292-4929-8929-929292929292"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusStopped,
		OperationRVersion: 72,
		RuntimeGeneration: 2,
		UpdatedUnix:       11,
		StoppedUnix:       10,
	})
	insertServiceCredentials(t, codespaceUUID)

	err := ReportRuntimeTransition(t.Context(), manager, ReportRuntimeTransitionOptions{
		CodespaceUUID:             codespaceUUID,
		RuntimeGeneration:         3,
		ObservedOperationRVersion: 72,
		RuntimeState:              RuntimeTransitionFailed,
	})
	require.NoError(t, err)

	row := loadServiceCodespace(t, codespaceUUID)
	require.Equal(t, codespace_model.StatusFailed, row.Status)
	require.EqualValues(t, 3, row.RuntimeGeneration)
	firstUpdated := row.UpdatedUnix
	assert.EqualValues(t, 10, row.StoppedUnix)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", codespaceUUID)
	assert.Contains(t, readServiceLog(t, row.LogFilename), "Gitea recorded runtime generation 3 as failed.")

	err = ReportRuntimeTransition(t.Context(), manager, ReportRuntimeTransitionOptions{
		CodespaceUUID:             codespaceUUID,
		RuntimeGeneration:         3,
		ObservedOperationRVersion: 72,
		RuntimeState:              RuntimeTransitionFailed,
	})
	require.NoError(t, err)
	assert.Equal(t, firstUpdated, loadServiceCodespace(t, codespaceUUID).UpdatedUnix)
}

func TestReportRuntimeTransitionAllowedWhenCodespaceDisabled(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Codespace.Enabled, false)()

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "96969696-9696-4969-8969-969696969696"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 76,
		RuntimeGeneration: 1,
	})
	insertServiceCredentials(t, codespaceUUID)

	err := ReportRuntimeTransition(t.Context(), manager, ReportRuntimeTransitionOptions{
		CodespaceUUID:             codespaceUUID,
		RuntimeGeneration:         2,
		ObservedOperationRVersion: 76,
		RuntimeState:              RuntimeTransitionStopped,
	})
	require.NoError(t, err)

	row := loadServiceCodespace(t, codespaceUUID)
	assert.Equal(t, codespace_model.StatusStopped, row.Status)
	assert.EqualValues(t, 2, row.RuntimeGeneration)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)
	assertServiceExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", codespaceUUID)
}

func TestReportRuntimeTransitionRejectsConflicts(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	activeUUID := "93939393-9393-4939-8939-939393939393"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 activeUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    73,
		OperationType:        codespace_model.OperationStop,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
		RuntimeGeneration:    1,
	})
	err := ReportRuntimeTransition(t.Context(), manager, ReportRuntimeTransitionOptions{
		CodespaceUUID:             activeUUID,
		RuntimeGeneration:         2,
		ObservedOperationRVersion: 73,
		RuntimeState:              RuntimeTransitionStopped,
	})
	require.ErrorIs(t, err, ErrRuntimeTransitionCurrentOperationConflict)
	assert.Equal(t, codespace_model.StatusRunning, loadServiceCodespace(t, activeUUID).Status)

	staleUUID := "94949494-9494-4949-8949-949494949494"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              staleUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 74,
		RuntimeGeneration: 5,
	})
	err = ReportRuntimeTransition(t.Context(), manager, ReportRuntimeTransitionOptions{
		CodespaceUUID:             staleUUID,
		RuntimeGeneration:         4,
		ObservedOperationRVersion: 74,
		RuntimeState:              RuntimeTransitionStopped,
	})
	var stale *StaleGenerationError
	require.ErrorAs(t, err, &stale)
	assert.EqualValues(t, 5, stale.CurrentGeneration)

	err = ReportRuntimeTransition(t.Context(), manager, ReportRuntimeTransitionOptions{
		CodespaceUUID:             staleUUID,
		RuntimeGeneration:         5,
		ObservedOperationRVersion: 74,
		RuntimeState:              RuntimeTransitionStopped,
	})
	require.ErrorIs(t, err, ErrRuntimeTransitionGenerationConflict)

	err = ReportRuntimeTransition(t.Context(), manager, ReportRuntimeTransitionOptions{
		CodespaceUUID:             staleUUID,
		RuntimeGeneration:         6,
		ObservedOperationRVersion: 73,
		RuntimeState:              RuntimeTransitionStopped,
	})
	assert.ErrorIs(t, err, ErrRuntimeTransitionStaleOperation)
}
