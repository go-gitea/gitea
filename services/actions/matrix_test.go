// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasMatrixWithNeeds(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		want     bool
	}{
		{
			name: "dynamic matrix referencing job output",
			strategy: `
matrix:
  version: ${{ fromJson(needs.generate.outputs.matrix) }}
`,
			want: true,
		},
		{
			name: "static matrix — no expression",
			strategy: `
matrix:
  os: [ubuntu-latest, windows-latest]
`,
			want: false,
		},
		{
			name: "value contains needs. but not inside expression",
			strategy: `
matrix:
  os: [needs.review-runner]
`,
			want: false,
		},
		{
			name: "needs. outside expression block",
			strategy: `
matrix:
  runner: needs.something-but-no-braces
`,
			want: false,
		},
		{
			name: "expression with needs but no .outputs.",
			strategy: `
matrix:
  version: ${{ needs.job1 }}
`,
			want: false,
		},
		{
			name: "empty strategy",
			strategy: "",
			want:     false,
		},
		{
			name: "strategy without matrix key",
			strategy: `
fail-fast: false
`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HasMatrixWithNeeds(tt.strategy))
		})
	}
}
