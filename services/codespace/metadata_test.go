// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"strings"
	"testing"

	"gitea.dev/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeRuntimeMetadataStrictFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name: "unknown top-level field",
			mutate: func(payload map[string]any) {
				payload["unexpected"] = true
			},
		},
		{
			name: "unknown removed runtime field",
			mutate: func(payload map[string]any) {
				payload["runtime"] = map[string]any{"unexpected": true}
			},
		},
		{
			name: "unknown boot field",
			mutate: func(payload map[string]any) {
				payload["boot"].(map[string]any)["unexpected"] = true
			},
		},
		{
			name: "unknown endpoint field",
			mutate: func(payload map[string]any) {
				payload["endpoints"].([]map[string]any)[0]["unexpected"] = true
			},
		},
		{
			name: "missing endpoint public",
			mutate: func(payload map[string]any) {
				delete(payload["endpoints"].([]map[string]any)[0], "public")
			},
		},
		{
			name: "workspace endpoint public",
			mutate: func(payload map[string]any) {
				payload["endpoints"].([]map[string]any)[0]["public"] = true
			},
		},
		{
			name: "duplicate endpoint id",
			mutate: func(payload map[string]any) {
				payload["endpoints"] = append(payload["endpoints"].([]map[string]any), map[string]any{
					"endpoint_id": "workspace",
					"label":       "Workspace Copy",
					"public":      false,
				})
			},
		},
		{
			name: "invalid endpoint id",
			mutate: func(payload map[string]any) {
				payload["endpoints"].([]map[string]any)[0]["endpoint_id"] = "Workspace"
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := normalizeRuntimeMetadata(runtimeMetadataJSON(t, tc.mutate))
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
			metadata, _, err := normalizeRuntimeMetadata(runtimeMetadataJSON(t, func(payload map[string]any) {
				payload["endpoints"].([]map[string]any)[0]["label"] = tc.label
				if tc.secondLabelOK {
					payload["endpoints"] = append(payload["endpoints"].([]map[string]any), map[string]any{
						"endpoint_id": "app",
						"label":       tc.label,
						"public":      true,
					})
				}
			}))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotEmpty(t, metadata.Endpoints)
			assert.Equal(t, tc.normalized, metadata.Endpoints[len(metadata.Endpoints)-1].Label)
			if tc.secondLabelOK {
				require.Len(t, metadata.Endpoints, 2)
				assert.Equal(t, metadata.Endpoints[0].Label, metadata.Endpoints[1].Label)
			}
		})
	}
}

func TestNormalizeRuntimeMetadataCanonicalizesEndpointOrder(t *testing.T) {
	first, firstHash, err := normalizeRuntimeMetadata(runtimeMetadataJSON(t, func(payload map[string]any) {
		payload["endpoints"] = []map[string]any{
			{"endpoint_id": "z-api", "label": "Z API", "public": true},
			{"endpoint_id": "workspace", "label": "Workspace", "public": false},
			{"endpoint_id": "app", "label": "App", "public": true},
		}
	}))
	require.NoError(t, err)

	second, secondHash, err := normalizeRuntimeMetadata(runtimeMetadataJSON(t, func(payload map[string]any) {
		payload["endpoints"] = []map[string]any{
			{"endpoint_id": "app", "label": "App", "public": true},
			{"endpoint_id": "workspace", "label": "Workspace", "public": false},
			{"endpoint_id": "z-api", "label": "Z API", "public": true},
		}
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

func TestNormalizeRuntimeMetadataBoot(t *testing.T) {
	t.Run("ready does not require runtime target", func(t *testing.T) {
		metadata, _, err := normalizeRuntimeMetadata(runtimeMetadataJSON(t, nil))
		require.NoError(t, err)
		assert.Equal(t, bootStageReady, metadata.Boot.Stage)
	})

	t.Run("non-ready accepts boot progress", func(t *testing.T) {
		metadata, _, err := normalizeRuntimeMetadata(runtimeMetadataJSON(t, func(payload map[string]any) {
			payload["boot"].(map[string]any)["stage"] = bootStagePublishRuntime
		}))
		require.NoError(t, err)
		assert.Equal(t, bootStagePublishRuntime, metadata.Boot.Stage)
	})

	t.Run("invalid utf8 raw json", func(t *testing.T) {
		_, _, err := normalizeRuntimeMetadata(string([]byte{0xff, '{', '}'}))
		require.Error(t, err)
	})
}

func runtimeMetadataJSON(t *testing.T, mutate func(map[string]any)) string {
	t.Helper()
	payload := map[string]any{
		"endpoints": []map[string]any{
			{
				"endpoint_id": "workspace",
				"label":       "Workspace",
				"public":      false,
			},
		},
		"boot": map[string]any{
			"operation_rversion": int64(1),
			"stage":              bootStageReady,
			"started_unix":       int64(100),
			"last_update_unix":   int64(101),
		},
	}
	if mutate != nil {
		mutate(payload)
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	return string(data)
}
