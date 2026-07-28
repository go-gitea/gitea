// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"html/template"
	"testing"
	"time"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

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
		"imports": ["_shared.AaAaAaAa.js"],
		"css": ["css/index.B3zrQPqD.css", "css/index-extra.CcCcCcCc.css"]
	},
	"_shared.AaAaAaAa.js": {
		"file": "js/shared.AaAaAaAa.js",
		"name": "shared",
		"css": ["css/shared.BbBbBbBb.css"]
	},
	"web_src/css/themes/theme-gitea-dark.css": {
		"file": "css/theme-gitea-dark.CyAaQnn5.css",
		"name": "theme-gitea-dark",
		"src": "web_src/css/themes/theme-gitea-dark.css",
		"isEntry": true
	}
}`

	t.Run("EmptyManifest", func(t *testing.T) {
		storeManifestFromBytes([]byte(``), 0, time.Now())
		// not in manifest -> custom theme fallback
		assert.Equal(t, "/assets/css/theme-gitea-dark.css", AssetURI("web_src/css/themes/theme-gitea-dark.css"))
		assert.Empty(t, entryStyleURLs("web_src/js/index.ts", "web_src/css/index.css"))
		assert.Empty(t, AssetNameFromHashedPath("css/no-such-file.css"))
	})

	t.Run("ParseManifest", func(t *testing.T) {
		storeManifestFromBytes([]byte(testManifest), 0, time.Now())

		// assets are addressed by their source path (the manifest key)
		assert.Equal(t, "/assets/js/index.C6Z2MRVQ.js", AssetURI("web_src/js/index.ts"))
		assert.Equal(t, "/assets/css/theme-gitea-dark.CyAaQnn5.css", AssetURI("web_src/css/themes/theme-gitea-dark.css"))

		// custom theme not in the manifest falls back to the static asset location
		assert.Equal(t, "/assets/css/theme-custom.css", AssetURI("web_src/css/themes/theme-custom.css"))

		// a JS entry's stylesheets: all of the entry's own CSS plus the CSS of statically-imported chunks
		assert.Equal(t, []string{
			"/assets/css/index.B3zrQPqD.css",
			"/assets/css/index-extra.CcCcCcCc.css",
			"/assets/css/shared.BbBbBbBb.css",
		}, entryStyleURLs("web_src/js/index.ts", "web_src/css/index.css"))
		assert.Equal(t, template.HTML(
			`<link rel="stylesheet" href="/assets/css/index.B3zrQPqD.css">`+
				`<link rel="stylesheet" href="/assets/css/index-extra.CcCcCcCc.css">`+
				`<link rel="stylesheet" href="/assets/css/shared.BbBbBbBb.css">`,
		), AssetCSSLinks("web_src/js/index.ts", "web_src/css/index.css"))

		// hashed output file -> entry name
		assert.Equal(t, "theme-gitea-dark", AssetNameFromHashedPath("css/theme-gitea-dark.CyAaQnn5.css"))
		assert.Empty(t, AssetNameFromHashedPath("css/no-such-file.css"))
	})
}
