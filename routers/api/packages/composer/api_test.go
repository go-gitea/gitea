// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package composer

import (
	"testing"
	"time"

	"gitea.dev/modules/json"
	composer_module "gitea.dev/modules/packages/composer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageVersionMetadataMarshalJSONWithRawMetadata(t *testing.T) {
	metadata := PackageVersionMetadata{
		Metadata: &composer_module.Metadata{},
		RawMetadata: map[string]any{
			"name":        "ignored/name",
			"version":     "ignored-version",
			"type":        "ignored-type",
			"description": "Dev branch package",
			"support": map[string]any{
				"issues": "https://example.com/issues",
				"source": "https://example.com/source",
			},
			"funding": []any{},
			"autoload": map[string]any{
				"psr-4": map[string]any{
					"Gitea\\Composer\\": "src/",
				},
			},
			"extra": map[string]any{
				"branch-alias": map[string]any{
					"dev-master": "1.0-dev",
				},
			},
		},
		Name:              "gitea/dev-only-package",
		Version:           "dev-master",
		VersionNormalized: "dev-master",
		Type:              "project",
		Created:           time.Unix(1, 0).UTC(),
		Dist: Dist{
			Type:      "zip",
			URL:       "https://example.com/archive/master.zip",
			Reference: "0123456789abcdef",
		},
		Source: Source{
			URL:       "https://example.com/repo",
			Type:      "git",
			Reference: "0123456789abcdef",
		},
	}

	bytes, err := metadata.MarshalJSON()
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(bytes, &result))

	assert.Equal(t, "gitea/dev-only-package", result["name"])
	assert.Equal(t, "dev-master", result["version"])
	assert.Equal(t, "project", result["type"])
	assert.Equal(t, "Dev branch package", result["description"])
	assert.Equal(t, map[string]any{"issues": "https://example.com/issues", "source": "https://example.com/source"}, result["support"])
	assert.Equal(t, []any{}, result["funding"])
	assert.Contains(t, result, "autoload")
	assert.Contains(t, result, "extra")

	dist, ok := result["dist"].(map[string]any)
	require.Truef(t, ok, "result[\"dist\"] is %T, want map[string]any", result["dist"])
	assert.Equal(t, "0123456789abcdef", dist["reference"])
	source, ok := result["source"].(map[string]any)
	require.Truef(t, ok, "result[\"source\"] is %T, want map[string]any", result["source"])
	assert.Equal(t, "0123456789abcdef", source["reference"])
}
