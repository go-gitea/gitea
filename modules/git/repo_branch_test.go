// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepository_GetBranches(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepository(bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	branches, countAll, err := bareRepo1.GetBranchNames(0, 2)

	assert.NoError(t, err)
	assert.Len(t, branches, 2)
	assert.EqualValues(t, 3, countAll)
	assert.ElementsMatch(t, []string{"branch1", "branch2"}, branches)

	branches, countAll, err = bareRepo1.GetBranchNames(0, 0)

	assert.NoError(t, err)
	assert.Len(t, branches, 3)
	assert.EqualValues(t, 3, countAll)
	assert.ElementsMatch(t, []string{"branch1", "branch2", "master"}, branches)

	branches, countAll, err = bareRepo1.GetBranchNames(5, 1)

	assert.NoError(t, err)
	assert.Len(t, branches, 0)
	assert.EqualValues(t, 3, countAll)
	assert.ElementsMatch(t, []string{}, branches)
}

func BenchmarkRepository_GetBranches(b *testing.B) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepository(bareRepo1Path)
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
