// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseManifest(t *testing.T) {
	manifest := []byte(`{
		"web_src/js/index.ts": {
			"file": "js/index.C6Z2MRVQ.js",
			"name": "index",
			"src": "web_src/js/index.ts",
			"isEntry": true,
			"css": ["css/index.B3zrQPqD.css"]
		},
		"web_src/js/standalone/swagger.ts": {
			"file": "js/swagger.SujiEmYM.js",
			"name": "swagger",
			"src": "web_src/js/standalone/swagger.ts",
			"isEntry": true,
			"css": ["css/swagger._-APWT_3.css"]
		},
		"web_src/css/themes/theme-gitea-dark.css": {
			"file": "css/theme-gitea-dark.CyAaQnn5.css",
			"name": "theme-gitea-dark",
			"src": "web_src/css/themes/theme-gitea-dark.css",
			"isEntry": true
		},
		"web_src/js/features/sharedworker.ts": {
			"file": "js/sharedworker.Dug1twio.js",
			"name": "sharedworker",
			"src": "web_src/js/features/sharedworker.ts",
			"isEntry": true
		},
		"_chunk.js": {
			"file": "js/chunk.abc123.js",
			"name": "chunk"
		}
	}`)

	paths := parseManifest(manifest)

	// JS entries
	assert.Equal(t, "js/index.C6Z2MRVQ.js", paths["js/index.js"])
	assert.Equal(t, "js/swagger.SujiEmYM.js", paths["js/swagger.js"])
	assert.Equal(t, "js/sharedworker.Dug1twio.js", paths["js/sharedworker.js"])

	// Associated CSS from JS entries
	assert.Equal(t, "css/index.B3zrQPqD.css", paths["css/index.css"])
	assert.Equal(t, "css/swagger._-APWT_3.css", paths["css/swagger.css"])

	// CSS-only entries
	assert.Equal(t, "css/theme-gitea-dark.CyAaQnn5.css", paths["css/theme-gitea-dark.css"])

	// Non-entry chunks should not be included
	assert.Empty(t, paths["js/chunk.js"])
}

func TestGetAssetPathFallback(t *testing.T) {
	// When manifest is not loaded, GetAssetPath should return the input as-is
	old := manifestPaths
	manifestPaths = make(map[string]string)
	defer func() { manifestPaths = old }()

	assert.Equal(t, "js/index.js", GetAssetPath("js/index.js"))
	assert.Equal(t, "css/theme-gitea-dark.css", GetAssetPath("css/theme-gitea-dark.css"))
}
