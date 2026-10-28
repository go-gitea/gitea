// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git_test

import (
	"testing"

	git_model "gitea.dev/models/git"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestIsUserAllowed(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	checkAllowed := func(pt *git_model.ProtectedTag, userID int64, check assert.BoolAssertionFunc) {
		allowed, err := git_model.IsUserAllowedModifyTag(t.Context(), pt, userID)
		assert.NoError(t, err)
		check(t, allowed)
	}
	checkAllowed(&git_model.ProtectedTag{}, 1, assert.False)

	pt := &git_model.ProtectedTag{
		AllowlistUserIDs: []int64{1},
	}
	checkAllowed(pt, 1, assert.True)
	checkAllowed(pt, 2, assert.False)

	pt = &git_model.ProtectedTag{
		AllowlistTeamIDs: []int64{1},
	}
	checkAllowed(pt, 1, assert.False)
	checkAllowed(pt, 2, assert.True)

	pt = &git_model.ProtectedTag{
		AllowlistUserIDs: []int64{1},
		AllowlistTeamIDs: []int64{1},
	}
	checkAllowed(pt, 1, assert.True)
	checkAllowed(pt, 2, assert.True)
}

func TestIsUserAllowedToControlTag(t *testing.T) {
	cases := []struct {
		name    string
		userid  int64
		allowed bool
	}{
		{
			name:    "test",
			userid:  1,
			allowed: true,
		},
		{
			name:    "test",
			userid:  3,
			allowed: true,
		},
		{
			name:    "gitea",
			userid:  1,
			allowed: true,
		},
		{
			name:    "gitea",
			userid:  3,
			allowed: false,
		},
		{
			name:    "test-gitea",
			userid:  1,
			allowed: true,
		},
		{
			name:    "test-gitea",
			userid:  3,
			allowed: false,
		},
		{
			name:    "gitea-test",
			userid:  1,
			allowed: true,
		},
		{
			name:    "gitea-test",
			userid:  3,
			allowed: true,
		},
		{
			name:    "v-1",
			userid:  1,
			allowed: false,
		},
		{
			name:    "v-1",
			userid:  2,
			allowed: true,
		},
		{
			name:    "release",
			userid:  1,
			allowed: false,
		},
	}

	for _, tc := range []struct {
		name     string
		patterns []string
	}{
		{"Glob", []string{`*gitea`, `v-*`, "release"}},
		{"Regex", []string{`/gitea\z/`, `/\Av-/`, "/release/"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			protectedTags := []*git_model.ProtectedTag{
				{
					NamePattern:      tc.patterns[0],
					AllowlistUserIDs: []int64{1},
				},
				{
					NamePattern:      tc.patterns[1],
					AllowlistUserIDs: []int64{2},
				},
				{
					NamePattern: tc.patterns[2],
				},
			}

			for n, c := range cases {
				isAllowed, err := git_model.IsUserAllowedToControlTag(t.Context(), protectedTags, c.name, c.userid)
				assert.NoError(t, err)
				assert.Equal(t, c.allowed, isAllowed, "case %d: error should match", n)
			}
		})
	}
}
