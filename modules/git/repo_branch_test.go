// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_GetBranches(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepositoryLocal(t.Context(), bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	branches, countAll, err := bareRepo1.GetBranchNames(t.Context(), 0, 2)

	assert.NoError(t, err)
	assert.Len(t, branches, 2)
	assert.Equal(t, 3, countAll)
	assert.ElementsMatch(t, []string{"master", "branch2"}, branches)

	branches, countAll, err = bareRepo1.GetBranchNames(t.Context(), 0, 0)

	assert.NoError(t, err)
	assert.Len(t, branches, 3)
	assert.Equal(t, 3, countAll)
	assert.ElementsMatch(t, []string{"master", "branch2", "branch1"}, branches)

	branches, countAll, err = bareRepo1.GetBranchNames(t.Context(), 5, 1)

	assert.NoError(t, err)
	assert.Empty(t, branches)
	assert.Equal(t, 3, countAll)
	assert.ElementsMatch(t, []string{}, branches)
}

func BenchmarkRepository_GetBranches(b *testing.B) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepositoryLocal(b.Context(), bareRepo1Path)
	if err != nil {
		b.Fatal(err)
	}
	defer bareRepo1.Close()

	for b.Loop() {
		_, _, err := bareRepo1.GetBranchNames(b.Context(), 0, 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestGetRefsBySha(t *testing.T) {
	bareRepo5Path := filepath.Join(testReposDir, "repo5_pulls")
	bareRepo5, err := OpenRepositoryLocal(t.Context(), bareRepo5Path)
	if err != nil {
		t.Fatal(err)
	}
	defer bareRepo5.Close()

	// do not exist
	branches, err := bareRepo5.GetRefsBySha(t.Context(), "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0", "")
	assert.NoError(t, err)
	assert.Empty(t, branches)

	// refs/pull/1/head
	branches, err = bareRepo5.GetRefsBySha(t.Context(), "c83380d7056593c51a699d12b9c00627bd5743e9", PullPrefix)
	assert.NoError(t, err)
	assert.Equal(t, []string{"refs/pull/1/head"}, branches)

	branches, err = bareRepo5.GetRefsBySha(t.Context(), "d8e0bbb45f200e67d9a784ce55bd90821af45ebd", BranchPrefix)
	assert.NoError(t, err)
	assert.Equal(t, []string{"refs/heads/master", "refs/heads/master-clone"}, branches)

	branches, err = bareRepo5.GetRefsBySha(t.Context(), "58a4bcc53ac13e7ff76127e0fb518b5262bf09af", BranchPrefix)
	assert.NoError(t, err)
	assert.Equal(t, []string{"refs/heads/test-patch-1"}, branches)
}

func BenchmarkGetRefsBySha(b *testing.B) {
	bareRepo5Path := filepath.Join(testReposDir, "repo5_pulls")
	bareRepo5, err := OpenRepositoryLocal(b.Context(), bareRepo5Path)
	if err != nil {
		b.Fatal(err)
	}
	defer bareRepo5.Close()

	_, _ = bareRepo5.GetRefsBySha(b.Context(), "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0", "")
	_, _ = bareRepo5.GetRefsBySha(b.Context(), "d8e0bbb45f200e67d9a784ce55bd90821af45ebd", "")
	_, _ = bareRepo5.GetRefsBySha(b.Context(), "c83380d7056593c51a699d12b9c00627bd5743e9", "")
	_, _ = bareRepo5.GetRefsBySha(b.Context(), "58a4bcc53ac13e7ff76127e0fb518b5262bf09af", "")
}

func TestRepository_Existence(t *testing.T) {
	for _, method := range []struct {
		name         string
		exists       func(*Repository, context.Context, string) bool
		branchExists bool
	}{
		{"IsObjectExist", (*Repository).IsObjectExist, false},
		{"IsReferenceExist", (*Repository).IsReferenceExist, true},
	} {
		t.Run(method.name, func(t *testing.T) {
			ctx := t.Context()
			repo, err := OpenRepositoryLocal(ctx, filepath.Join(testReposDir, "repo1_bare"))
			require.NoError(t, err)
			defer repo.Close()

			for _, testCase := range []struct {
				name string
				arg  string
				want bool
			}{
				{"empty", "", false},
				{"branch", "master", method.branchExists},
				{"commit hash", "ce064814f4a0d337b333e646ece456cd39fab612", true},
				{"short commit hash", "ce06481", true},
				{"blob hash", "153f451b9ee7fa1da317ab17a127e9fd9d384310", true},
				{"short blob hash", "153f451", true},
			} {
				t.Run(testCase.name, func(t *testing.T) {
					assert.Equal(t, testCase.want, method.exists(repo, ctx, testCase.arg))
				})
			}
		})
	}
}
