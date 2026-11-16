// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"strings"
	"testing"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeRuntimeMetadataProtoValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*codespacev1.RuntimeMetadata)
	}{
		{
			name: "missing boot",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.Boot = nil
			},
		},
		{
			name: "invalid stage",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.Boot.Stage = codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_UNSPECIFIED
			},
		},
		{
			name: "duplicate workspace endpoint",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.Endpoints[0].EndpointId = "workspace"
			},
		},
		{
			name: "public workspace endpoint",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.Endpoints[len(metadata.Endpoints)-1].Public = true
			},
		},
		{
			name: "workspace endpoint port",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.Endpoints[len(metadata.Endpoints)-1].Port = 13337
			},
		},
		{
			name: "duplicate endpoint id",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.Endpoints = append(metadata.Endpoints, &codespacev1.RuntimeEndpoint{
					EndpointId: "web",
					Label:      "Web Copy",
				})
			},
		},
		{
			name: "invalid endpoint id",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.Endpoints[0].EndpointId = "Workspace"
			},
		},
		{
			name: "invalid endpoint port",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.Endpoints[0].Port = 65536
			},
		},
		{
			name: "negative resource usage",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.ResourceUsage.Memory.UsedBytes = -1
			},
		},
		{
			name: "missing disk capacity",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.ResourceUsage.Disk.LimitBytes = 0
			},
		},
		{
			name: "missing ready workspace folder",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.WorkspaceFolder = ""
			},
		},
		{
			name: "relative workspace folder",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.WorkspaceFolder = "workspaces/project"
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			metadata := serviceRuntimeMetadataProto(1, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY, []*codespacev1.RuntimeEndpoint{{
				EndpointId: "web",
				Label:      "Web",
				Public:     false,
			}})
			tc.mutate(metadata)
			_, _, err := normalizeRuntimeMetadata(metadata)
			require.Error(t, err)
		})
	}

	t.Run("workspace endpoint is optional", func(t *testing.T) {
		metadata := serviceRuntimeMetadataProto(1, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY, []*codespacev1.RuntimeEndpoint{{
			EndpointId: "web",
			Label:      "Web",
			Public:     false,
		}})
		metadata.Endpoints = metadata.Endpoints[:1]
		normalized, _, err := normalizeRuntimeMetadata(metadata)
		require.NoError(t, err)
		require.Len(t, normalized.Endpoints, 1)
	})

	t.Run("workspace folder is optional while booting", func(t *testing.T) {
		metadata := serviceRuntimeMetadataProto(1, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_PREPARE_RUNTIME, nil)
		normalized, _, err := normalizeRuntimeMetadata(metadata)
		require.NoError(t, err)
		assert.Empty(t, normalized.WorkspaceFolder)
	})
}

func TestNormalizeRuntimeMetadataLabelBoundaries(t *testing.T) {
	valid64 := strings.Repeat("界", 64)
	tests := []struct {
		name          string
		label         string
		wantErr       bool
		normalized    string
		secondLabelOK bool
	}{
		{name: "trim unicode whitespace", label: " \t界面\n", normalized: "界面"},
		{name: "one character", label: "界", normalized: "界"},
		{name: "sixty four characters", label: valid64, normalized: valid64},
		{name: "sixty five characters", label: strings.Repeat("界", 65), wantErr: true},
		{name: "blank after trim", label: " \t\n", wantErr: true},
		{name: "control character", label: "bad\nlabel", wantErr: true},
		{name: "less-than", label: "bad<label", wantErr: true},
		{name: "greater-than", label: "bad>label", wantErr: true},
		{name: "duplicate label allowed", label: "服务", normalized: "服务", secondLabelOK: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			endpoints := []*codespacev1.RuntimeEndpoint{{
				EndpointId: "web",
				Label:      tc.label,
				Public:     false,
			}}
			if tc.secondLabelOK {
				endpoints = append(endpoints, &codespacev1.RuntimeEndpoint{
					EndpointId: "app",
					Label:      tc.label,
					Public:     true,
				})
			}
			metadata, _, err := normalizeRuntimeMetadata(serviceRuntimeMetadataProto(1, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY, endpoints))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			web, found := metadata.endpointByID("web")
			require.True(t, found)
			assert.Equal(t, tc.normalized, web.Label)
			if tc.secondLabelOK {
				require.Len(t, metadata.Endpoints, 3)
				app, found := metadata.endpointByID("app")
				require.True(t, found)
				assert.Equal(t, web.Label, app.Label)
			}
		})
	}
}

func TestNormalizeRuntimeMetadataCanonicalizesEndpointOrder(t *testing.T) {
	first, firstHash, err := normalizeRuntimeMetadata(serviceRuntimeMetadataProto(1, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "z-api", Label: "Z API", Public: true, Port: 8080},
		{EndpointId: "app", Label: "App", Public: true, Port: 3000},
	}))
	require.NoError(t, err)

	second, secondHash, err := normalizeRuntimeMetadata(serviceRuntimeMetadataProto(1, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "app", Label: "App", Public: true, Port: 3000},
		{EndpointId: "z-api", Label: "Z API", Public: true, Port: 8080},
	}))
	require.NoError(t, err)

	assert.Equal(t, []string{"app", "workspace", "z-api"}, []string{
		first.Endpoints[0].EndpointID,
		first.Endpoints[1].EndpointID,
		first.Endpoints[2].EndpointID,
	})
	assert.Equal(t, first, second)
	assert.Equal(t, firstHash, secondHash)
	assert.EqualValues(t, 3000, first.Endpoints[0].Port)
}

func TestNormalizeRuntimeMetadataBootAndResourceUsage(t *testing.T) {
	input := serviceRuntimeMetadataProto(1, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_PUBLISH_READY, nil)
	input.ResourceUsage.Cpu.LimitMillicores = 0
	input.ResourceUsage.Memory.LimitBytes = 0
	metadata, hash, err := normalizeRuntimeMetadata(input)
	require.NoError(t, err)
	assert.Equal(t, bootStagePublishReady, metadata.Boot.Stage)
	assert.Equal(t, "/workspaces/project", metadata.WorkspaceFolder)
	assert.EqualValues(t, 125, metadata.ResourceUsage.CPU.UsedMillicores)
	assert.EqualValues(t, 256*1024*1024, metadata.ResourceUsage.Memory.UsedBytes)
	assert.EqualValues(t, 512*1024*1024, metadata.ResourceUsage.Disk.UsedBytes)
	assert.Zero(t, metadata.ResourceUsage.CPU.LimitMillicores)
	assert.Zero(t, metadata.ResourceUsage.Memory.LimitBytes)
	input.ResourceUsage.Cpu.UsedMillicores++
	_, nextHash, err := normalizeRuntimeMetadata(input)
	require.NoError(t, err)
	assert.Equal(t, hash, nextHash)
	input.ResourceUsage = nil
	metadata, nextHash, err = normalizeRuntimeMetadata(input)
	require.NoError(t, err)
	assert.Equal(t, hash, nextHash)
	assert.Zero(t, metadata.ResourceUsage.ObservedUnix)
}

func serviceRuntimeMetadataEntry(t *testing.T, operationRVersion int64, endpoints []*codespacev1.RuntimeEndpoint) runtimeMetadataCacheEntry {
	t.Helper()
	metadata, hash, err := normalizeRuntimeMetadata(serviceRuntimeMetadataProto(operationRVersion, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY, endpoints))
	require.NoError(t, err)
	return runtimeMetadataCacheEntry{
		Metadata:    metadata,
		Generation:  1,
		ContentHash: hash,
	}
}

func serviceRuntimeMetadataProto(operationRVersion int64, stage codespacev1.RuntimeBootStage, endpoints []*codespacev1.RuntimeEndpoint) *codespacev1.RuntimeMetadata {
	metadata := &codespacev1.RuntimeMetadata{
		Endpoints: endpoints,
		Boot: &codespacev1.RuntimeBoot{
			OperationRversion: operationRVersion,
			Stage:             stage,
			StartedUnix:       100,
			LastUpdateUnix:    101,
		},
		ResourceUsage: serviceRuntimeMetadataResourceUsage(),
	}
	if stage == codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_PUBLISH_READY || stage == codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY {
		metadata.Endpoints = append(metadata.Endpoints, &codespacev1.RuntimeEndpoint{
			EndpointId: workspaceEndpointID,
			Label:      workspaceEndpointLabel,
		})
		metadata.WorkspaceFolder = "/workspaces/project"
	}
	return metadata
}

func serviceRuntimeMetadataResourceUsage() *codespacev1.RuntimeResourceUsage {
	return &codespacev1.RuntimeResourceUsage{
		Cpu:          &codespacev1.RuntimeCPUUsage{UsedMillicores: 125, LimitMillicores: 1000},
		Memory:       &codespacev1.RuntimeMemoryUsage{UsedBytes: 256 * 1024 * 1024, LimitBytes: 1024 * 1024 * 1024},
		Disk:         &codespacev1.RuntimeDiskUsage{UsedBytes: 512 * 1024 * 1024, LimitBytes: 10 * 1024 * 1024 * 1024},
		ObservedUnix: 101,
	}
}
