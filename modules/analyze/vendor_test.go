// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package analyze

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsVendor(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		// Actual vendor directories should be detected
		{"vendor/file.go", true},
		{"vendor/github.com/pkg/errors/errors.go", true},
		{"Vendor/file.go", true},
		{"src/vendor/file.go", true},
		{"vendors/file.go", true},
		{"node_modules/package/index.js", true},
		{"src/node_modules/package/file.js", true},
		{"bower_components/package/file.js", true},
		{"Godeps/file.go", true},
		{"Godeps/_workspace/src/pkg/file.go", true},
		{"third_party/lib/file.go", true},
		{"3rdparty/lib/file.go", true},
		{"external/lib/file.go", true},
		{"externals/lib/file.go", true},

		// Directories with similar names should NOT be detected as vendored
		{"myvendor/file.go", false},
		{"vendor_old/file.go", false},
		{"external_lib/file.go", false},
		{"node_modules_backup/file.js", false},

		// Git-related files should NOT be detected as vendored
		{".gitignore", false},
		{".gitattributes", false},
		{".gitmodules", false},
		{"src/.gitignore", false},
		{".github/workflows/ci.yml", false},
		{".github/CODEOWNERS", false},

		// Regular source files should NOT be detected as vendored
		{"main.go", false},
		{"src/index.js", false},
		{"index.js", false},
		{"app.js", false},

		// Test data and other directories should NOT be detected as vendored
		{"testdata/file.txt", false},
		{"tests/fixtures/file.txt", false},

		// Minified files should NOT be detected as vendored
		{"app.min.js", false},
		{"styles.min.css", false},

		// Config files should NOT be detected as vendored
		{".editorconfig", false},
		{".rubocop.yml", false},
		{"configure", false},
		{"config.guess", false},

		// IDE/editor directories should NOT be detected as vendored
		{".vscode/settings.json", false},

		// Build output directories should NOT be detected as vendored
		{"dist/bundle.js", false},
		{"cache/file.txt", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsVendor(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}
