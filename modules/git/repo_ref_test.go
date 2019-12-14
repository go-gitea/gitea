// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepository_GetRefs(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepository(bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	refs, err := bareRepo1.GetRefs()

	assert.NoError(t, err)
	assert.Len(t, refs, 5)

	expectedRefs := []string{
		BranchPrefix + "branch1",
		BranchPrefix + "branch2",
		BranchPrefix + "master",
		TagPrefix + "test",
		NotesRef,
	}

	for _, ref := range refs {
		assert.Contains(t, expectedRefs, ref.Name)
	}
}

func TestRepository_GetRefsFiltered(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepository(bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	refs, err := bareRepo1.GetRefsFiltered(TagPrefix)

	assert.NoError(t, err)
	if assert.Len(t, refs, 1) {
		assert.Equal(t, TagPrefix+"test", refs[0].Name)
		assert.Equal(t, "tag", refs[0].Type)
		assert.Equal(t, "3ad28a9149a2864384548f3d17ed7f38014c9e8a", refs[0].Object.String())
	}
}
