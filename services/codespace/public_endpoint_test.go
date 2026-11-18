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
	codespaceID := int64(1041)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 12,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, serviceRuntimeMetadataEntry(t, 12, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "app-3000", Label: "App", Public: true},
		{EndpointId: "private-api", Label: "API", Public: false},
	})))

	result, err := ValidatePublicEndpoint(t.Context(), manager, ValidatePublicEndpointOptions{
		CodespaceID: codespaceID,
		EndpointID:  "app-3000",
	})
	require.NoError(t, err)
	assert.NotNil(t, result.GetAllowed())
}

func TestValidatePublicEndpointDeniesPrivateMissingAndWorkspace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1042)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 13,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, serviceRuntimeMetadataEntry(t, 13, []*codespacev1.RuntimeEndpoint{
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
			CodespaceID: codespaceID,
			EndpointID:  tc.endpointID,
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
	activeID := int64(1117)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   activeID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    14,
		OperationTrigger:     codespace_model.OperationTriggerIdle,
		OperationCreatedUnix: time.Now().Unix(),
	})
	require.NoError(t, putRuntimeMetadataEntry(activeID, serviceRuntimeMetadataEntry(t, 14, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "app-3000", Label: "App", Public: true},
	})))

	result, err := ValidatePublicEndpoint(t.Context(), manager, ValidatePublicEndpointOptions{
		CodespaceID: activeID,
		EndpointID:  "app-3000",
	})
	require.NoError(t, err)
	assert.Equal(t, PublicEndpointDeniedActiveOperation, result.GetDenied().GetCategory())

	missingMetadataID := int64(1118)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                missingMetadataID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 15,
	})
	result, err = ValidatePublicEndpoint(t.Context(), manager, ValidatePublicEndpointOptions{
		CodespaceID: missingMetadataID,
		EndpointID:  "app-3000",
	})
	require.NoError(t, err)
	assert.Equal(t, PublicEndpointDeniedMetadataRebuilding, result.GetDenied().GetCategory())

	result, err = ValidatePublicEndpoint(t.Context(), otherManager, ValidatePublicEndpointOptions{
		CodespaceID: missingMetadataID,
		EndpointID:  "app-3000",
	})
	require.NoError(t, err)
	assert.Equal(t, PublicEndpointDeniedManagerMismatch, result.GetDenied().GetCategory())

	manager.RuntimeState = codespace_model.ManagerRuntimeStateRecovering
	_, err = db.GetEngine(t.Context()).ID(manager.ID).Cols("runtime_state").Update(manager)
	require.NoError(t, err)
	result, err = ValidatePublicEndpoint(t.Context(), manager, ValidatePublicEndpointOptions{
		CodespaceID: missingMetadataID,
		EndpointID:  "app-3000",
	})
	require.NoError(t, err)
	assert.Equal(t, PublicEndpointDeniedManagerOffline, result.GetDenied().GetCategory())
}
