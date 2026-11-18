// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListGovernanceCodespacesAndActions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	runningID := int64(1050)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                runningID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 31,
	})
	queued := &codespace_model.Codespace{
		Status:               codespace_model.StatusCreating,
		OperationRVersion:    1,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: 1,
	}
	insertServiceCodespace(t, 0, queued)

	list, err := ListGovernanceCodespaces(t.Context(), GovernanceListOptions{
		ManagerID: manager.ID,
		Page:      1,
		PageSize:  30,
	})
	require.NoError(t, err)
	require.Len(t, list.Rows, 1)
	assert.EqualValues(t, 1, list.Total)
	assert.Equal(t, runningID, list.Rows[0].ID)
	assert.True(t, list.Rows[0].CanStop)
	assert.True(t, list.Rows[0].CanForceDelete)
	assert.Equal(t, "User One", list.Rows[0].UserDisplayName)
	assert.Equal(t, "user2/repo2", list.Rows[0].RepoFullName)
	assert.Equal(t, "manager", list.Rows[0].ManagerDisplayName)
	assert.Equal(t, "main", list.Rows[0].RefName)

	unassigned, err := ListGovernanceCodespaces(t.Context(), GovernanceListOptions{
		Unassigned: true,
		Page:       1,
		PageSize:   30,
	})
	require.NoError(t, err)
	require.Len(t, unassigned.Rows, 1)
	assert.Equal(t, queued.ID, unassigned.Rows[0].ID)
	assert.True(t, unassigned.Rows[0].CanDelete)
	assert.True(t, unassigned.Rows[0].CanForceDelete)
	assert.Equal(t, managerDisplayPending, unassigned.Rows[0].ManagerRuntimeState)
	_, err = DeleteGovernanceCodespace(t.Context(), GovernanceActionOptions{CodespaceID: queued.ID, Unassigned: true})
	require.NoError(t, err)
	assertServiceNotExists(t, new(codespace_model.Codespace), "id = ?", queued.ID)

	_, err = StopGovernanceCodespace(t.Context(), GovernanceActionOptions{CodespaceID: runningID, ManagerID: manager.ID})
	require.NoError(t, err)
	row := loadServiceCodespace(t, runningID)
	assert.Equal(t, codespace_model.OperationStop, codespace_model.ActiveOperationType(row))
	assert.True(t, codespace_model.IsOperationQueued(row))
	assert.EqualValues(t, 32, row.OperationRVersion)
	_, err = db.DeleteByID[codespace_model.Manager](t.Context(), manager.ID)
	require.NoError(t, err)
	orphaned, err := ListGovernanceCodespaces(t.Context(), GovernanceListOptions{Unassigned: true, Page: 1, PageSize: 30})
	require.NoError(t, err)
	require.Len(t, orphaned.Rows, 1)
	assert.False(t, orphaned.Rows[0].CanStop)
	assert.False(t, orphaned.Rows[0].CanDelete)
	assert.True(t, orphaned.Rows[0].CanForceDelete)
	orphanTarget := GovernanceActionOptions{CodespaceID: runningID, Unassigned: true}
	_, err = DeleteGovernanceCodespace(t.Context(), orphanTarget)
	require.ErrorIs(t, err, ErrGovernanceStateUnavailable)
	require.NoError(t, ForceDeleteCodespace(t.Context(), orphanTarget))
	assertServiceNotExists(t, new(codespace_model.Codespace), "id = ?", runningID)
}

func TestGovernanceActionRequiresListedManager(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	otherManager := insertServiceManager(t)
	codespaceID := int64(1051)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 34,
	})

	_, err := StopGovernanceCodespace(t.Context(), GovernanceActionOptions{
		CodespaceID: codespaceID,
		ManagerID:   otherManager.ID,
	})
	require.ErrorIs(t, err, ErrGovernanceNotFound)
	assert.Equal(t, codespace_model.StatusRunning, loadServiceCodespace(t, codespaceID).Status)
}

func TestForceDeleteCodespaceRemovesLocalState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceID := int64(1052)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusDeleting,
		OperationRVersion: 36,
	})
	insertServiceCredentials(t, codespaceID)

	err := ForceDeleteCodespace(t.Context(), GovernanceActionOptions{CodespaceID: codespaceID, ManagerID: manager.ID})
	require.NoError(t, err)
	assertServiceNotExists(t, new(codespace_model.Codespace), "id = ?", codespaceID)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_id = ?", codespaceID)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_id = ?", codespaceID)
}
