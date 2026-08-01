// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListGovernanceCodespacesAndActions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	runningUUID := "31313131-3131-4131-8131-313131313131"
	unboundUUID := "33333333-3333-4333-8333-333333333333"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              runningUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 31,
	})
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		UUID:            unboundUUID,
		Status:          codespace_model.StatusCreating,
		OperationType:   codespace_model.OperationCreate,
		OperationStatus: codespace_model.OperationStatusQueued,
	})

	list, err := ListGovernanceCodespaces(t.Context(), GovernanceListOptions{
		ManagerID: manager.ID,
		Page:      1,
		PageSize:  30,
	})
	require.NoError(t, err)
	require.Len(t, list.Rows, 1)
	assert.EqualValues(t, 1, list.Total)
	rows := governanceRowsByUUID(list.Rows)
	assert.True(t, rows[runningUUID].CanStop)
	assert.True(t, rows[runningUUID].CanForceDelete)
	assert.Equal(t, "user2/repo2", rows[runningUUID].RepoFullName)
	assert.Equal(t, "main", rows[runningUUID].RefName)

	unassigned, err := ListGovernanceCodespaces(t.Context(), GovernanceListOptions{
		Unassigned: true,
		Page:       1,
		PageSize:   30,
	})
	require.NoError(t, err)
	require.Len(t, unassigned.Rows, 1)
	assert.Equal(t, unboundUUID, unassigned.Rows[0].UUID)
	assert.True(t, unassigned.Rows[0].CanDelete)
	assert.True(t, unassigned.Rows[0].CanForceDelete)
	assert.Equal(t, managerDisplayPending, unassigned.Rows[0].ManagerRuntimeState)

	_, err = StopGovernanceCodespace(t.Context(), GovernanceActionOptions{CodespaceUUID: runningUUID})
	require.NoError(t, err)
	row := loadServiceCodespace(t, runningUUID)
	assert.Equal(t, codespace_model.OperationStop, row.OperationType)
	assert.Equal(t, codespace_model.OperationStatusQueued, row.OperationStatus)
	assert.EqualValues(t, 32, row.OperationRVersion)
}

func TestGovernanceActionRequiresListedManager(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	otherManager := insertServiceManager(t)
	codespaceUUID := "34343434-3434-4434-8434-343434343434"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 34,
	})

	_, err := StopGovernanceCodespace(t.Context(), GovernanceActionOptions{
		CodespaceUUID: codespaceUUID,
		ManagerID:     otherManager.ID,
	})
	require.ErrorIs(t, err, ErrGovernanceNotFound)
	assert.Equal(t, codespace_model.StatusRunning, loadServiceCodespace(t, codespaceUUID).Status)
}

func TestForceDeleteCodespaceRemovesLocalState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceUUID := "36363636-3636-4636-8636-363636363636"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusDeleting,
		OperationRVersion: 36,
		OperationType:     codespace_model.OperationDelete,
		OperationStatus:   codespace_model.OperationStatusRunning,
	})
	insertServiceCredentials(t, codespaceUUID)

	err := ForceDeleteCodespace(t.Context(), GovernanceActionOptions{CodespaceUUID: codespaceUUID})
	require.NoError(t, err)
	assertServiceNotExists(t, new(codespace_model.Codespace), "uuid = ?", codespaceUUID)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", codespaceUUID)
}

func governanceRowsByUUID(rows []*GovernanceView) map[string]*GovernanceView {
	result := make(map[string]*GovernanceView, len(rows))
	for _, row := range rows {
		result[row.UUID] = row
	}
	return result
}
