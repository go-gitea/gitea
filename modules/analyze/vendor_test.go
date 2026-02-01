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
		// Actual vendor directories should be detected (go-enry behavior)
		{"vendor/file.go", true},
		{"vendor/github.com/pkg/errors/errors.go", true},
		{"node_modules/package/index.js", true},
		{"Godeps/_workspace/src/pkg/file.go", true},

		// Git-related files should NOT be detected as vendored (override)
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
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsVendor(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}
