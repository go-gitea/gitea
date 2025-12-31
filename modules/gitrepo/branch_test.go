// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepository_GetBranches(t *testing.T) {
	storage := &mockRepository{path: "repo1_bare"}

	branches, countAll, err := GetBranchNames(t.Context(), storage, 0, 2)

	assert.NoError(t, err)
	assert.Len(t, branches, 2)
	assert.Equal(t, 3, countAll)
	assert.ElementsMatch(t, []string{"master", "branch2"}, branches)

	branches, countAll, err = GetBranchNames(t.Context(), storage, 0, 0)

	assert.NoError(t, err)
	assert.Len(t, branches, 3)
	assert.Equal(t, 3, countAll)
	assert.ElementsMatch(t, []string{"master", "branch2", "branch1"}, branches)

	branches, countAll, err = GetBranchNames(t.Context(), storage, 5, 1)

	assert.NoError(t, err)
	assert.Empty(t, branches)
	assert.Equal(t, 3, countAll)
	assert.ElementsMatch(t, []string{}, branches)
}

func BenchmarkRepository_GetBranches(b *testing.B) {
	storage := &mockRepository{path: "repo1_bare"}

	for b.Loop() {
		_, _, err := GetBranchNames(b.Context(), storage, 0, 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}
