// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_GetSubmoduleCommits(t *testing.T) {
	testRepoPath := filepath.Join(testReposDir, "repo4_submodules")
	submodules, err := GetTemplateSubmoduleCommits(DefaultContext, testRepoPath)
	require.NoError(t, err)

	assert.Len(t, submodules, 2)

	assert.EqualValues(t, "<°)))><", submodules[0].Path)
	assert.EqualValues(t, "d2932de67963f23d43e1c7ecf20173e92ee6c43c", submodules[0].Commit)

	assert.EqualValues(t, "libtest", submodules[1].Path)
	assert.EqualValues(t, "1234567890123456789012345678901234567890", submodules[1].Commit)
}
