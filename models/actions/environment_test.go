// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
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
		{"no policy allows any ref", "", "refs/heads/feature", true},
		{"branch matches a pattern", "main\nrelease/*", "refs/heads/release/1.0", true},
		{"branch matches no pattern", "main\nrelease/*", "refs/heads/feature", false},
		{"tag is matched without its prefix", "v*", "refs/tags/v1.0", true},
		// The `on:` branch-filter dialect, so a pattern can be copied from one to the other.
		{"super wildcard spans slashes", "release/**", "refs/heads/release/1/0", true},
		{"a catch-all allows a pull request ref", "*", "refs/pull/3/head", true},
		{"a branch policy denies a pull request ref", "main", "refs/pull/3/head", false},
		// A policy that cannot be evaluated has to deny, or a typo silently grants every ref the rest of the list would refuse.
		{"a malformed pattern denies", "main\n[unterminated", "refs/heads/main", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := &ActionEnvironment{AllowedBranchPatterns: tt.patterns}
			assert.Equal(t, tt.want, env.MatchesRef(tt.ref))
		})
	}
}

func TestValidateEnvironmentName(t *testing.T) {
	for _, name := range []string{"production", "staging-2", "with space", "Ünicode"} {
		require.NoError(t, ValidateEnvironmentName(name), name)
	}
	// Rejected so the name survives a round trip through a URL path segment and a template link.
	for _, name := range []string{"", "a/b", "a#b", "a?b", "a%b", "a\\b", " leading", "trailing ", "new\nline", ".", ".."} {
		require.Error(t, ValidateEnvironmentName(name), "%q must be rejected", name)
	}
}

func TestJoinBranchPatterns(t *testing.T) {
	// Blank entries are dropped, and a comma stays part of the pattern because branch names may contain one.
	got, err := JoinBranchPatterns([]string{" main ", "", "release/*", "a,b"})
	require.NoError(t, err)
	assert.Equal(t, "main\nrelease/*\na,b", got)
	assert.Equal(t, []string{"main", "release/*", "a,b"}, SplitBranchPatterns(got))

	_, err = JoinBranchPatterns([]string{"["})
	require.Error(t, err, "a pattern that cannot compile must be rejected on write")
}
