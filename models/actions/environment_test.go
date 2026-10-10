// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentMatchesRef(t *testing.T) {
	tests := []struct {
		name     string
		patterns string
		ref      string
		want     bool
	}{
		{"no policy allows any ref", "", "refs/pull/3/head", true},
		{"bare pattern matches a branch", "main\nrelease/*", "refs/heads/release/1.0", true},
		{"branch matching no pattern is denied", "main\nrelease/*", "refs/heads/feature", false},
		{"bare pattern does not match a tag of the same name", "main", "refs/tags/main", false},
		{"bare tag-like pattern does not match a look-alike branch", "v*", "refs/tags/v1.0", false},
		{"refs/tags pattern matches a tag", "refs/tags/v*", "refs/tags/v1.0", true},
		{"refs/tags pattern does not match a branch", "refs/tags/v*", "refs/heads/v-evil", false},
		{"explicit refs/heads pattern matches a branch", "refs/heads/main", "refs/heads/main", true},
		{"a catch-all bare pattern does not match a pull ref", "*", "refs/pull/3/head", false},
		{"a malformed pattern denies", "main\n[unterminated", "refs/heads/main", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := &ActionEnvironment{AllowedBranchPatterns: tt.patterns}
			assert.Equal(t, tt.want, env.MatchesRef(tt.ref))
		})
	}
}

func TestEnvironmentNameAndPatterns(t *testing.T) {
	for _, name := range []string{"production", "with space", "a/b", strings.Repeat("é", EnvironmentNameMaxLength)} {
		require.NoError(t, ValidateEnvironmentName(name), name)
	}
	for _, name := range []string{"", strings.Repeat("é", EnvironmentNameMaxLength+1)} {
		require.Error(t, ValidateEnvironmentName(name))
	}

	got, err := JoinBranchPatterns([]string{" main ", "", "refs/tags/v*", "a,b"})
	require.NoError(t, err)
	assert.Equal(t, []string{"main", "refs/tags/v*", "a,b"}, SplitBranchPatterns(got))
	assert.Equal(t, []string{}, SplitBranchPatterns(""))

	_, err = JoinBranchPatterns([]string{"["})
	require.Error(t, err)
}
