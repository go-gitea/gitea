// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_IsObjectExist(t *testing.T) {
	repo, err := OpenRepository(t.Context(), filepath.Join(testReposDir, "repo1_bare"))
	require.NoError(t, err)
	defer repo.Close()

	// FIXME: Inconsistent behavior between gogit and nogogit editions
	// See the comment of IsObjectExist in gogit edition for more details.
	supportShortHash := !isGogit

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "empty",
			arg:  "",
			want: false,
		},
		{
			name: "branch",
			arg:  "master",
			want: false,
		},
		{
			name: "commit hash",
			arg:  "ce064814f4a0d337b333e646ece456cd39fab612",
			want: true,
		},
		{
			name: "short commit hash",
			arg:  "ce06481",
			want: supportShortHash,
		},
		{
			name: "blob hash",
			arg:  "153f451b9ee7fa1da317ab17a127e9fd9d384310",
			want: true,
		},
		{
			name: "short blob hash",
			arg:  "153f451",
			want: supportShortHash,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, repo.IsObjectExist(tt.arg))
		})
	}
}

func TestRepository_IsReferenceExist(t *testing.T) {
	repo, err := OpenRepository(t.Context(), filepath.Join(testReposDir, "repo1_bare"))
	require.NoError(t, err)
	defer repo.Close()

	// FIXME: Inconsistent behavior between gogit and nogogit editions
	// See the comment of IsReferenceExist in gogit edition for more details.
	supportBlobHash := !isGogit

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "empty",
			arg:  "",
			want: false,
		},
		{
			name: "branch",
			arg:  "master",
			want: true,
		},
		{
			name: "commit hash",
			arg:  "ce064814f4a0d337b333e646ece456cd39fab612",
			want: true,
		},
		{
			name: "short commit hash",
			arg:  "ce06481",
			want: true,
		},
		{
			name: "blob hash",
			arg:  "153f451b9ee7fa1da317ab17a127e9fd9d384310",
			want: supportBlobHash,
		},
		{
			name: "short blob hash",
			arg:  "153f451",
			want: supportBlobHash,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, repo.IsReferenceExist(tt.arg))
		})
	}
}
