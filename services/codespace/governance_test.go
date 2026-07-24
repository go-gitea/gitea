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

func TestListGovernanceCodespacesScopesAndActions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	orgManager := insertServiceGovernanceManager(t, 3)
	globalManager := insertServiceGovernanceManager(t, 0)
	orgUUID := "31313131-3131-4131-8131-313131313131"
	globalUUID := "32323232-3232-4232-8232-323232323232"
	unboundUUID := "33333333-3333-4333-8333-333333333333"
	insertServiceCodespace(t, orgManager.ID, &codespace_model.Codespace{
		UUID:              orgUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 31,
	})
	insertServiceCodespace(t, globalManager.ID, &codespace_model.Codespace{
		UUID:              globalUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 32,
	})
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		UUID:            unboundUUID,
		Status:          codespace_model.StatusCreating,
		OperationType:   codespace_model.OperationCreate,
		OperationStatus: codespace_model.OperationStatusQueued,
	})

	orgList, err := ListGovernanceCodespaces(t.Context(), GovernanceListOptions{
		Scope:   GovernanceScopeOrganization,
		OwnerID: 3,
	})
	require.NoError(t, err)
	require.Len(t, orgList.Rows, 1)
	assert.Equal(t, orgUUID, orgList.Rows[0].UUID)
	assert.True(t, orgList.Rows[0].CanStop)
	assert.True(t, orgList.Rows[0].CanDelete)
	assert.False(t, orgList.Rows[0].CanForceDelete)

	siteList, err := ListGovernanceCodespaces(t.Context(), GovernanceListOptions{Scope: GovernanceScopeSite})
	require.NoError(t, err)
	require.Len(t, siteList.Rows, 3)
	siteRows := governanceRowsByUUID(siteList.Rows)
	assert.True(t, siteRows[unboundUUID].CanDelete)
	assert.True(t, siteRows[unboundUUID].CanForceDelete)
	assert.Equal(t, managerDisplayPending, siteRows[unboundUUID].ManagerRuntimeState)
	assert.True(t, siteRows[globalUUID].CanStop)
	assert.True(t, siteRows[globalUUID].CanForceDelete)
}

func TestGovernanceActionsUseScopeAndLifecycleRules(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	orgManager := insertServiceGovernanceManager(t, 3)
	globalManager := insertServiceGovernanceManager(t, 0)
	orgUUID := "34343434-3434-4434-8434-343434343434"
	globalUUID := "35353535-3535-4535-8535-353535353535"
	insertServiceCodespace(t, orgManager.ID, &codespace_model.Codespace{
		UUID:              orgUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 34,
	})
	insertServiceCodespace(t, globalManager.ID, &codespace_model.Codespace{
		UUID:              globalUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 35,
	})

	_, err := StopGovernanceCodespace(t.Context(), GovernanceActionOptions{
		Scope:         GovernanceScopeOrganization,
		OwnerID:       3,
		CodespaceUUID: globalUUID,
	})
	require.ErrorIs(t, err, ErrGovernanceNotFound)

	_, err = StopGovernanceCodespace(t.Context(), GovernanceActionOptions{
		Scope:         GovernanceScopeOrganization,
		OwnerID:       3,
		CodespaceUUID: orgUUID,
	})
	require.NoError(t, err)
	row := loadServiceCodespace(t, orgUUID)
	assert.Equal(t, codespace_model.OperationStop, row.OperationType)
	assert.Equal(t, codespace_model.OperationStatusQueued, row.OperationStatus)
	assert.EqualValues(t, 35, row.OperationRVersion)
}

func TestForceDeleteCodespaceRemovesLocalState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceGovernanceManager(t, 0)
	codespaceUUID := "36363636-3636-4636-8636-363636363636"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusDeleting,
		OperationRVersion: 36,
		OperationType:     codespace_model.OperationDelete,
		OperationStatus:   codespace_model.OperationStatusRunning,
	})
	insertServiceCredentials(t, codespaceUUID)

	err := ForceDeleteCodespace(t.Context(), GovernanceActionOptions{
		Scope:         GovernanceScopeSite,
		CodespaceUUID: codespaceUUID,
	})
	require.NoError(t, err)
	assertServiceNotExists(t, new(codespace_model.Codespace), "uuid = ?", codespaceUUID)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", codespaceUUID)
}

func insertServiceGovernanceManager(t *testing.T, ownerID int64) *codespace_model.Manager {
	t.Helper()
	manager := insertServiceManager(t)
	manager.OwnerID = ownerID
	manager.Name = "manager"
	_, err := db.GetEngine(t.Context()).ID(manager.ID).Cols("owner_id", "name").Update(manager)
	require.NoError(t, err)
	markServiceManagerOnline(t, manager, `["default"]`)
	return manager
}

func governanceRowsByUUID(rows []*GovernanceView) map[string]*GovernanceView {
	result := make(map[string]*GovernanceView, len(rows))
	for _, row := range rows {
		result[row.UUID] = row
	}
	return result
}
