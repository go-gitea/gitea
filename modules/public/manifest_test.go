// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestViteManifest(t *testing.T) {
	defer test.MockVariableValue(&setting.IsProd, true)()

	const testManifest = `{
	"web_src/js/index.ts": {
		"file": "js/index.C6Z2MRVQ.js",
		"name": "index",
		"src": "web_src/js/index.ts",
		"isEntry": true,
		"css": ["css/index.B3zrQPqD.css"]
	},
	"web_src/css/themes/theme-gitea-dark.css": {
		"file": "css/theme-gitea-dark.CyAaQnn5.css",
		"name": "theme-gitea-dark",
		"src": "web_src/css/themes/theme-gitea-dark.css",
		"isEntry": true
	},
	"web_src/js/features/eventsource.sharedworker.ts": {
		"file": "js/eventsource.sharedworker.Dug1twio.js",
		"name": "eventsource.sharedworker",
		"src": "web_src/js/features/eventsource.sharedworker.ts",
		"isEntry": true
	},
	"_chunk.js": {
		"file": "js/chunk.abc123.js",
		"name": "chunk"
	}
}`

	t.Run("EmptyManifest", func(t *testing.T) {
		storeManifestFromBytes([]byte(``), 0, time.Now())
		assert.Equal(t, "/assets/js/index.js", AssetURI("js/index.js"))
		assert.Equal(t, "/assets/css/theme-gitea-dark.css", AssetURI("css/theme-gitea-dark.css"))
		assert.Equal(t, "", AssetNameFromHashedPath("css/no-such-file.css"))
	})

	t.Run("ParseManifest", func(t *testing.T) {
		storeManifestFromBytes([]byte(testManifest), 0, time.Now())
		paths, names := manifestData.Load().paths, manifestData.Load().names

		// JS entries
		assert.Equal(t, "js/index.C6Z2MRVQ.js", paths["js/index.js"])
		assert.Equal(t, "js/eventsource.sharedworker.Dug1twio.js", paths["js/eventsource.sharedworker.js"])

		// Associated CSS from JS entries
		assert.Equal(t, "css/index.B3zrQPqD.css", paths["css/index.css"])

		// CSS-only entries
		assert.Equal(t, "css/theme-gitea-dark.CyAaQnn5.css", paths["css/theme-gitea-dark.css"])

		// Non-entry chunks should not be included
		assert.Empty(t, paths["js/chunk.js"])

		// Names: hashed path -> entry name
		assert.Equal(t, "index", names["js/index.C6Z2MRVQ.js"])
		assert.Equal(t, "index", names["css/index.B3zrQPqD.css"])
		assert.Equal(t, "theme-gitea-dark", names["css/theme-gitea-dark.CyAaQnn5.css"])
		assert.Equal(t, "eventsource.sharedworker", names["js/eventsource.sharedworker.Dug1twio.js"])

		// Test Asset related functions
		assert.Equal(t, "/assets/js/index.C6Z2MRVQ.js", AssetURI("js/index.js"))
		assert.Equal(t, "/assets/css/theme-gitea-dark.CyAaQnn5.css", AssetURI("css/theme-gitea-dark.css"))
		assert.Equal(t, "theme-gitea-dark", AssetNameFromHashedPath("css/theme-gitea-dark.CyAaQnn5.css"))
	})
}
