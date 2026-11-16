// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePublicEndpointAllowsPublicReadyEndpoint(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "61616161-6161-4616-8616-616161616161"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 12,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 12, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "app-3000", Label: "App", Public: true},
		{EndpointId: "private-api", Label: "API", Public: false},
	})))

	result, err := ValidatePublicEndpoint(t.Context(), manager, ValidatePublicEndpointOptions{
		CodespaceUUID: codespaceUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	assert.NotNil(t, result.GetAllowed())
}

func TestValidatePublicEndpointDeniesPrivateMissingAndWorkspace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "62626262-6262-4626-8626-626262626262"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 13,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 13, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "private-api", Label: "API", Public: false},
	})))

	for _, tc := range []struct {
		endpointID string
		category   string
	}{
		{"private-api", PublicEndpointDeniedEndpointNotPublic},
		{"missing", PublicEndpointDeniedEndpointNotPublic},
		{"workspace", PublicEndpointDeniedInvalidEndpoint},
	} {
		result, err := ValidatePublicEndpoint(t.Context(), manager, ValidatePublicEndpointOptions{
			CodespaceUUID: codespaceUUID,
			EndpointID:    tc.endpointID,
		})
		require.NoError(t, err)
		assert.Equal(t, tc.category, result.GetDenied().GetCategory())
	}
}

func TestValidatePublicEndpointDeniesStateAndMetadata(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	otherManager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	markServiceManagerOnline(t, otherManager, `[{"tag":"default"}]`)
	activeUUID := "63636363-6363-4636-8636-636363636363"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 activeUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    14,
		OperationTrigger:     codespace_model.OperationTriggerIdle,
		OperationCreatedUnix: time.Now().Unix(),
	})
	require.NoError(t, putRuntimeMetadataEntry(activeUUID, serviceRuntimeMetadataEntry(t, 14, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "app-3000", Label: "App", Public: true},
	})))

	result, err := ValidatePublicEndpoint(t.Context(), manager, ValidatePublicEndpointOptions{
		CodespaceUUID: activeUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	assert.Equal(t, PublicEndpointDeniedActiveOperation, result.GetDenied().GetCategory())

	missingMetadataUUID := "64646464-6464-4646-8646-646464646464"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              missingMetadataUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 15,
	})
	result, err = ValidatePublicEndpoint(t.Context(), manager, ValidatePublicEndpointOptions{
		CodespaceUUID: missingMetadataUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	assert.Equal(t, PublicEndpointDeniedMetadataRebuilding, result.GetDenied().GetCategory())

	result, err = ValidatePublicEndpoint(t.Context(), otherManager, ValidatePublicEndpointOptions{
		CodespaceUUID: missingMetadataUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	assert.Equal(t, PublicEndpointDeniedManagerMismatch, result.GetDenied().GetCategory())

	manager.RuntimeState = codespace_model.ManagerRuntimeStateRecovering
	_, err = db.GetEngine(t.Context()).ID(manager.ID).Cols("runtime_state").Update(manager)
	require.NoError(t, err)
	result, err = ValidatePublicEndpoint(t.Context(), manager, ValidatePublicEndpointOptions{
		CodespaceUUID: missingMetadataUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	assert.Equal(t, PublicEndpointDeniedManagerOffline, result.GetDenied().GetCategory())
}
