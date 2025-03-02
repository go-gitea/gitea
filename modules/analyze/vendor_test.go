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
		{"cache/", true},
		{"random/cache/", true},
		{"cache", false},
		{"dependencies/", true},
		{"Dependencies/", true},
		{"dependency/", false},
		{"dist/", true},
		{"dist", false},
		{"random/dist/", true},
		{"random/dist", false},
		{"deps/", true},
		{"configure", true},
		{"a/configure", true},
		{"config.guess", true},
		{"config.guess/", false},
		{".vscode/", true},
		{"doc/_build/", true},
		{"a/docs/_build/", true},
		{"a/dasdocs/_build-vsdoc.js", true},
		{"a/dasdocs/_build-vsdoc.j", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsVendor(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}
