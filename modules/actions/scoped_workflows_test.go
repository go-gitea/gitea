// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsWorkflowInDirs(t *testing.T) {
	tests := []struct {
		name     string
		dirs     []string
		path     string
		expected bool
	}{
		{
			name:     "default scoped dir with yml",
			dirs:     []string{".gitea/scoped_workflows", ".github/scoped_workflows"},
			path:     ".gitea/scoped_workflows/security.yml",
			expected: true,
		},
		{
			name:     "default scoped dir with yaml",
			dirs:     []string{".gitea/scoped_workflows", ".github/scoped_workflows"},
			path:     ".github/scoped_workflows/lint.yaml",
			expected: true,
		},
		{
			name:     "normal workflow path is not a scoped workflow",
			dirs:     []string{".gitea/scoped_workflows"},
			path:     ".gitea/workflows/ci.yml",
			expected: false,
		},
		{
			name:     "non-yaml file",
			dirs:     []string{".gitea/scoped_workflows"},
			path:     ".gitea/scoped_workflows/readme.md",
			expected: false,
		},
		{
			name:     "feature disabled (no scoped dirs)",
			dirs:     []string{},
			path:     ".gitea/scoped_workflows/security.yml",
			expected: false,
		},
		{
			name:     "directory boundary",
			dirs:     []string{".gitea/scoped_workflows"},
			path:     ".gitea/scoped_workflows2/security.yml",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isWorkflowInDirs(tt.path, tt.dirs))
		})
	}
}
