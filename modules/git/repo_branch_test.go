// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_GetBranches(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := openRepositoryWithDefaultContext(bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	branches, countAll, err := bareRepo1.GetBranchNames(0, 2)

	assert.NoError(t, err)
	assert.Len(t, branches, 2)
	assert.EqualValues(t, 3, countAll)
	assert.ElementsMatch(t, []string{"master", "branch2"}, branches)

	branches, countAll, err = bareRepo1.GetBranchNames(0, 0)

	assert.NoError(t, err)
	assert.Len(t, branches, 3)
	assert.EqualValues(t, 3, countAll)
	assert.ElementsMatch(t, []string{"master", "branch2", "branch1"}, branches)

	branches, countAll, err = bareRepo1.GetBranchNames(5, 1)

	assert.NoError(t, err)
	assert.Empty(t, branches)
	assert.EqualValues(t, 3, countAll)
	assert.ElementsMatch(t, []string{}, branches)
}

func BenchmarkRepository_GetBranches(b *testing.B) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := openRepositoryWithDefaultContext(bareRepo1Path)
	if err != nil {
		b.Fatal(err)
	}
	defer bareRepo1.Close()

	for i := 0; i < b.N; i++ {
		_, _, err := bareRepo1.GetBranchNames(0, 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestGetRefsBySha(t *testing.T) {
	bareRepo5Path := filepath.Join(testReposDir, "repo5_pulls")
	bareRepo5, err := OpenRepository(DefaultContext, bareRepo5Path)
	if err != nil {
		t.Fatal(err)
	}
	defer bareRepo5.Close()

	// do not exist
	branches, err := bareRepo5.GetRefsBySha("8006ff9adbf0cb94da7dad9e537e53817f9fa5c0", "")
	assert.NoError(t, err)
	assert.Empty(t, branches)

	// refs/pull/1/head
	branches, err = bareRepo5.GetRefsBySha("c83380d7056593c51a699d12b9c00627bd5743e9", PullPrefix)
	assert.NoError(t, err)
	assert.EqualValues(t, []string{"refs/pull/1/head"}, branches)

	branches, err = bareRepo5.GetRefsBySha("d8e0bbb45f200e67d9a784ce55bd90821af45ebd", BranchPrefix)
	assert.NoError(t, err)
	assert.EqualValues(t, []string{"refs/heads/master", "refs/heads/master-clone"}, branches)

	branches, err = bareRepo5.GetRefsBySha("58a4bcc53ac13e7ff76127e0fb518b5262bf09af", BranchPrefix)
	assert.NoError(t, err)
	assert.EqualValues(t, []string{"refs/heads/test-patch-1"}, branches)
}

func BenchmarkGetRefsBySha(b *testing.B) {
	bareRepo5Path := filepath.Join(testReposDir, "repo5_pulls")
	bareRepo5, err := OpenRepository(DefaultContext, bareRepo5Path)
	if err != nil {
		b.Fatal(err)
	}
	defer bareRepo5.Close()

	_, _ = bareRepo5.GetRefsBySha("8006ff9adbf0cb94da7dad9e537e53817f9fa5c0", "")
	_, _ = bareRepo5.GetRefsBySha("d8e0bbb45f200e67d9a784ce55bd90821af45ebd", "")
	_, _ = bareRepo5.GetRefsBySha("c83380d7056593c51a699d12b9c00627bd5743e9", "")
	_, _ = bareRepo5.GetRefsBySha("58a4bcc53ac13e7ff76127e0fb518b5262bf09af", "")
}

func TestRepository_IsObjectExist(t *testing.T) {
	repo, err := openRepositoryWithDefaultContext(filepath.Join(testReposDir, "repo1_bare"))
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
	repo, err := openRepositoryWithDefaultContext(filepath.Join(testReposDir, "repo1_bare"))
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
