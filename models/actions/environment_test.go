// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentMatchesBranch(t *testing.T) {
	tests := []struct {
		name              string
		protectedBranches string
		ref               string
		want              bool
	}{
		{
			name:              "empty policy allows everything",
			protectedBranches: "",
			ref:               "refs/heads/feature",
			want:              true,
		},
		{
			name:              "exact match",
			protectedBranches: "main",
			ref:               "refs/heads/main",
			want:              true,
		},
		{
			name:              "no match",
			protectedBranches: "main",
			ref:               "refs/heads/develop",
			want:              false,
		},
		{
			name:              "glob wildcard matches",
			protectedBranches: "release/*",
			ref:               "refs/heads/release/1.0",
			want:              true,
		},
		{
			name:              "glob wildcard no match",
			protectedBranches: "release/*",
			ref:               "refs/heads/main",
			want:              false,
		},
		{
			name:              "multiple patterns comma-separated, first matches",
			protectedBranches: "main, staging",
			ref:               "refs/heads/main",
			want:              true,
		},
		{
			name:              "multiple patterns comma-separated, second matches",
			protectedBranches: "main, staging",
			ref:               "refs/heads/staging",
			want:              true,
		},
		{
			name:              "multiple patterns, none match",
			protectedBranches: "main, staging",
			ref:               "refs/heads/feature/x",
			want:              false,
		},
		{
			name:              "glob and exact combo",
			protectedBranches: "main, release/*",
			ref:               "refs/heads/release/2.0",
			want:              true,
		},
		{
			name:              "ref without refs/heads/ prefix",
			protectedBranches: "main",
			ref:               "main",
			want:              true,
		},
		{
			name:              "wildcard does not cross path separator",
			protectedBranches: "release/*",
			ref:               "refs/heads/release/1.0/hotfix",
			want:              false,
		},
		{
			name:              "malformed pattern is skipped, later valid pattern still matches",
			protectedBranches: "[invalid, main",
			ref:               "refs/heads/main",
			want:              true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := &ActionEnvironment{ProtectedBranches: tt.protectedBranches}
			assert.Equal(t, tt.want, env.MatchesBranch(tt.ref))
		})
	}
}

func TestValidateProtectedBranches(t *testing.T) {
	require.NoError(t, ValidateProtectedBranches(""))
	require.NoError(t, ValidateProtectedBranches("main, release/*, feature/**"))
	require.Error(t, ValidateProtectedBranches("main, [invalid"))
}

func TestEnvironmentCRUD(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	const repoID int64 = 4

	t.Run("Insert and Get", func(t *testing.T) {
		env, err := InsertEnvironment(ctx, repoID, "production", "main")
		require.NoError(t, err)
		assert.Positive(t, env.ID)
		assert.Equal(t, "production", env.Name)
		assert.Equal(t, "main", env.ProtectedBranches)

		got, err := GetEnvironmentByRepoAndName(ctx, repoID, "production")
		require.NoError(t, err)
		assert.Equal(t, env.ID, got.ID)

		gotByID, err := GetEnvironmentByID(ctx, env.ID)
		require.NoError(t, err)
		assert.Equal(t, "production", gotByID.Name)
	})

	t.Run("Update", func(t *testing.T) {
		env, err := InsertEnvironment(ctx, repoID, "staging", "develop")
		require.NoError(t, err)

		env.ProtectedBranches = "staging,develop"
		require.NoError(t, UpdateEnvironment(ctx, env))

		got, err := GetEnvironmentByID(ctx, env.ID)
		require.NoError(t, err)
		assert.Equal(t, "staging,develop", got.ProtectedBranches)
	})

	t.Run("NotFound error for unknown env", func(t *testing.T) {
		_, err := GetEnvironmentByRepoAndName(ctx, repoID, "nonexistent")
		require.ErrorIs(t, err, ErrEnvironmentNotFound{Name: "nonexistent"})
	})
}
