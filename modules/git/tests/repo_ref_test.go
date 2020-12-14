// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tests

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
	"github.com/stretchr/testify/assert"
)

func TestRepository_GetRefs(t *testing.T) {
	RunTestPerProvider(t, func(service service.GitService, t *testing.T) {
		bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
		bareRepo1, err := service.OpenRepository(bareRepo1Path)
		assert.NoError(t, err)
		defer bareRepo1.Close()

		refs, err := bareRepo1.GetRefs()

		assert.NoError(t, err)
		assert.Len(t, refs, 5)

		expectedRefs := []string{
			git.BranchPrefix + "branch1",
			git.BranchPrefix + "branch2",
			git.BranchPrefix + "master",
			git.TagPrefix + "test",
			git.NotesRef,
		}

		for _, ref := range refs {
			assert.Contains(t, expectedRefs, ref.Name)
		}
	})
}

func TestRepository_GetRefsFiltered(t *testing.T) {
	RunTestPerProvider(t, func(service service.GitService, t *testing.T) {

		bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
		bareRepo1, err := service.OpenRepository(bareRepo1Path)
		assert.NoError(t, err)
		defer bareRepo1.Close()

		refs, err := bareRepo1.GetRefsFiltered(git.TagPrefix)

		assert.NoError(t, err)
		if assert.Len(t, refs, 1) {
			assert.Equal(t, git.TagPrefix+"test", refs[0].Name())
			assert.Equal(t, "tag", refs[0].Type())
			assert.Equal(t, "3ad28a9149a2864384548f3d17ed7f38014c9e8a", refs[0].ID().String())
		}
	})
}
