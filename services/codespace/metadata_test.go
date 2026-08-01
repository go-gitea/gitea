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
			name: "missing workspace endpoint",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.Endpoints = metadata.Endpoints[:1]
			},
		},
		{
			name: "public workspace endpoint",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.Endpoints[len(metadata.Endpoints)-1].Public = true
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
			name: "missing resource usage",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.ResourceUsage = nil
			},
		},
		{
			name: "negative resource usage",
			mutate: func(metadata *codespacev1.RuntimeMetadata) {
				metadata.ResourceUsage.Memory.UsedBytes = -1
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			metadata := metadataProtoForTest(t, 1, bootStageReady, []map[string]any{{
				"endpoint_id": "web",
				"label":       "Web",
				"public":      false,
			}})
			tc.mutate(metadata)
			_, _, err := normalizeRuntimeMetadata(metadata)
			require.Error(t, err)
		})
	}
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
			endpoints := []map[string]any{{
				"endpoint_id": "web",
				"label":       tc.label,
				"public":      false,
			}}
			if tc.secondLabelOK {
				endpoints = append(endpoints, map[string]any{
					"endpoint_id": "app",
					"label":       tc.label,
					"public":      true,
				})
			}
			metadata, _, err := normalizeRuntimeMetadata(metadataProtoForTest(t, 1, bootStageReady, endpoints))
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
	first, firstHash, err := normalizeRuntimeMetadata(metadataProtoForTest(t, 1, bootStageReady, []map[string]any{
		{"endpoint_id": "z-api", "label": "Z API", "public": true},
		{"endpoint_id": "app", "label": "App", "public": true},
	}))
	require.NoError(t, err)

	second, secondHash, err := normalizeRuntimeMetadata(metadataProtoForTest(t, 1, bootStageReady, []map[string]any{
		{"endpoint_id": "app", "label": "App", "public": true},
		{"endpoint_id": "z-api", "label": "Z API", "public": true},
	}))
	require.NoError(t, err)

	assert.Equal(t, []string{"app", "workspace", "z-api"}, []string{
		first.Endpoints[0].EndpointID,
		first.Endpoints[1].EndpointID,
		first.Endpoints[2].EndpointID,
	})
	assert.Equal(t, first, second)
	assert.Equal(t, firstHash, secondHash)
}

func TestNormalizeRuntimeMetadataBootAndResourceUsage(t *testing.T) {
	metadata, _, err := normalizeRuntimeMetadata(metadataProtoForTest(t, 1, bootStagePublishReady, []map[string]any{}))
	require.NoError(t, err)
	assert.Equal(t, bootStagePublishReady, metadata.Boot.Stage)
	assert.EqualValues(t, 125, metadata.ResourceUsage.CPU.UsedMillicores)
	assert.EqualValues(t, 256*1024*1024, metadata.ResourceUsage.Memory.UsedBytes)
	assert.EqualValues(t, 512*1024*1024, metadata.ResourceUsage.Disk.UsedBytes)
}

func metadataProtoForTest(t *testing.T, operationRVersion int64, stage string, endpoints []map[string]any) *codespacev1.RuntimeMetadata {
	t.Helper()
	return serviceRuntimeMetadataProto(t, operationRVersion, stage, endpoints)
}
