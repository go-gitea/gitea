// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"gitea.dev/modules/git/gitcmd"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoIsEmpty(t *testing.T) {
	emptyRepo2Path := filepath.Join(testReposDir, "repo2_empty")
	repo, err := OpenRepositoryLocal(t.Context(), emptyRepo2Path)
	assert.NoError(t, err)
	defer repo.Close()
	isEmpty, err := repo.IsEmpty(t.Context())
	assert.NoError(t, err)
	assert.True(t, isEmpty)
}

func TestRepoObjectFormat(t *testing.T) {
	for _, objectFormat := range DefaultFeatures().SupportedObjectFormats {
		repoPath := filepath.Join(t.TempDir(), "repo.git")
		require.NoError(t, InitRepositoryLocal(t.Context(), repoPath, true, objectFormat.Name()))
		if objectFormat != Sha1ObjectFormat {
			_, _, err := gitcmd.NewCommand("tag").AddDynamicArguments(Sha1ObjectFormat.EmptyTree().String(), objectFormat.EmptyTree().String()).WithDir(repoPath).RunStdString(t.Context())
			require.NoError(t, err)
		}
		repo, err := OpenRepositoryLocal(t.Context(), repoPath)
		require.NoError(t, err)
		detected, err := repo.GetObjectFormat(t.Context())
		repo.Close()
		require.NoError(t, err)
		assert.Equal(t, objectFormat, detected)
	}
}
