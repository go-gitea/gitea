// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"gitea.dev/modules/json"
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePublicEndpointAllowsPublicReadyEndpoint(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "61616161-6161-4616-8616-616161616161"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 12,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 12, []map[string]any{
		{"endpoint_id": "app-3000", "label": "App", "public": true},
		{"endpoint_id": "private-api", "label": "API", "public": false},
	})))

	result, err := ValidatePublicEndpoint(t.Context(), manager, ValidatePublicEndpointOptions{
		CodespaceUUID: codespaceUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Empty(t, result.DeniedCategory)
}

func TestValidatePublicEndpointDeniesPrivateMissingAndWorkspace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "62626262-6262-4626-8626-626262626262"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 13,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 13, []map[string]any{
		{"endpoint_id": "private-api", "label": "API", "public": false},
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
		assert.False(t, result.Allowed)
		assert.Equal(t, tc.category, result.DeniedCategory)
	}
}

func TestValidatePublicEndpointDeniesStateAndMetadata(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	otherManager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	markServiceManagerOnline(t, otherManager, `["default"]`)
	activeUUID := "63636363-6363-4636-8636-636363636363"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 activeUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    14,
		OperationType:        codespace_model.OperationStop,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerIdle,
		OperationCreatedUnix: time.Now().Unix(),
	})
	require.NoError(t, putRuntimeMetadataEntry(activeUUID, serviceRuntimeMetadataEntry(t, 14, []map[string]any{
		{"endpoint_id": "app-3000", "label": "App", "public": true},
	})))

	result, err := ValidatePublicEndpoint(t.Context(), manager, ValidatePublicEndpointOptions{
		CodespaceUUID: activeUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	assert.Equal(t, PublicEndpointDeniedActiveOperation, result.DeniedCategory)

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
	assert.Equal(t, PublicEndpointDeniedMetadataRebuilding, result.DeniedCategory)

	result, err = ValidatePublicEndpoint(t.Context(), otherManager, ValidatePublicEndpointOptions{
		CodespaceUUID: missingMetadataUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	assert.Equal(t, PublicEndpointDeniedManagerMismatch, result.DeniedCategory)

	manager.RuntimeState = codespace_model.ManagerRuntimeStateRecovering
	_, err = db.GetEngine(t.Context()).ID(manager.ID).Cols("runtime_state").Update(manager)
	require.NoError(t, err)
	result, err = ValidatePublicEndpoint(t.Context(), manager, ValidatePublicEndpointOptions{
		CodespaceUUID: missingMetadataUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	assert.Equal(t, PublicEndpointDeniedManagerOffline, result.DeniedCategory)
}

func serviceRuntimeMetadataEntry(t *testing.T, operationRVersion int64, endpoints []map[string]any) runtimeMetadataCacheEntry {
	t.Helper()
	payload := map[string]any{
		"endpoints": endpoints,
		"boot": map[string]any{
			"operation_rversion": operationRVersion,
			"stage":              bootStageReady,
			"started_unix":       int64(100),
			"last_update_unix":   int64(101),
		},
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	metadata, hash, err := normalizeRuntimeMetadata(string(data))
	require.NoError(t, err)
	return runtimeMetadataCacheEntry{
		Metadata:         metadata,
		Generation:       1,
		ContentHash:      hash,
		LastReportedUnix: time.Now().Unix(),
	}
}
