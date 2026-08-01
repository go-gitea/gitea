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
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 12, []map[string]any{
		{"endpoint_id": "app-3000", "label": "App", "public": true},
		{"endpoint_id": "private-api", "label": "API", "public": false},
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

func serviceRuntimeMetadataEntry(t *testing.T, operationRVersion int64, endpoints []map[string]any) runtimeMetadataCacheEntry {
	t.Helper()
	metadata, hash, err := normalizeRuntimeMetadata(serviceRuntimeMetadataProto(t, operationRVersion, bootStageReady, endpoints))
	require.NoError(t, err)
	return runtimeMetadataCacheEntry{
		Metadata:         metadata,
		Generation:       1,
		ContentHash:      hash,
		LastReportedUnix: time.Now().Unix(),
	}
}

func serviceRuntimeMetadataProto(t *testing.T, operationRVersion int64, stage string, endpoints []map[string]any) *codespacev1.RuntimeMetadata {
	t.Helper()
	metadataEndpoints := make([]*codespacev1.RuntimeEndpoint, 0, len(endpoints)+1)
	for _, endpoint := range endpoints {
		metadataEndpoints = append(metadataEndpoints, &codespacev1.RuntimeEndpoint{
			EndpointId: endpoint["endpoint_id"].(string),
			Label:      endpoint["label"].(string),
			Public:     endpoint["public"].(bool),
		})
	}
	if stage == bootStagePublishReady || stage == bootStageReady {
		metadataEndpoints = append(metadataEndpoints, &codespacev1.RuntimeEndpoint{
			EndpointId: workspaceEndpointID,
			Label:      workspaceEndpointLabel,
		})
	}
	return &codespacev1.RuntimeMetadata{
		Endpoints: metadataEndpoints,
		Boot: &codespacev1.RuntimeBoot{
			OperationRversion: operationRVersion,
			Stage:             serviceRuntimeMetadataBootStage(t, stage),
			StartedUnix:       100,
			LastUpdateUnix:    101,
		},
		ResourceUsage: serviceRuntimeMetadataResourceUsage(),
	}
}

func serviceRuntimeMetadataResourceUsage() *codespacev1.RuntimeResourceUsage {
	return &codespacev1.RuntimeResourceUsage{
		Cpu:          &codespacev1.RuntimeCPUUsage{UsedMillicores: 125, LimitMillicores: 1000},
		Memory:       &codespacev1.RuntimeMemoryUsage{UsedBytes: 256 * 1024 * 1024, LimitBytes: 1024 * 1024 * 1024},
		Disk:         &codespacev1.RuntimeDiskUsage{UsedBytes: 512 * 1024 * 1024, LimitBytes: 10 * 1024 * 1024 * 1024},
		ObservedUnix: 101,
	}
}

func serviceRuntimeMetadataBootStage(t *testing.T, stage string) codespacev1.RuntimeBootStage {
	t.Helper()
	switch stage {
	case bootStagePrepareRuntime:
		return codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_PREPARE_RUNTIME
	case bootStageInitializeSystem:
		return codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_INITIALIZE_SYSTEM
	case bootStagePrepareWorkspace:
		return codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_PREPARE_WORKSPACE
	case bootStageStartEnvironment:
		return codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_START_ENVIRONMENT
	case bootStagePublishReady:
		return codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_PUBLISH_READY
	case bootStageReady:
		return codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY
	default:
		t.Fatalf("unknown runtime metadata stage %q", stage)
		return codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_UNSPECIFIED
	}
}
