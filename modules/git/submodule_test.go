// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTemplateSubmoduleCommits(t *testing.T) {
	testRepoPath := filepath.Join(testReposDir, "repo4_submodules")
	submodules, err := GetTemplateSubmoduleCommits(DefaultContext, testRepoPath)
	require.NoError(t, err)

	assert.Len(t, submodules, 2)

	assert.EqualValues(t, "<°)))><", submodules[0].Path)
	assert.EqualValues(t, "d2932de67963f23d43e1c7ecf20173e92ee6c43c", submodules[0].Commit)

	assert.EqualValues(t, "libtest", submodules[1].Path)
	assert.EqualValues(t, "1234567890123456789012345678901234567890", submodules[1].Commit)
}

func TestAddTemplateSubmoduleIndexes(t *testing.T) {
	ctx := t.Context()
	tmpDir := t.TempDir()
	var err error
	_, _, err = NewCommand("init").RunStdString(ctx, &RunOpts{Dir: tmpDir})
	require.NoError(t, err)
	_ = os.Mkdir(filepath.Join(tmpDir, "new-dir"), 0o755)
	err = AddTemplateSubmoduleIndexes(ctx, tmpDir, []TemplateSubmoduleCommit{{Path: "new-dir", Commit: "1234567890123456789012345678901234567890"}})
	require.NoError(t, err)
	_, _, err = NewCommand("add", "--all").RunStdString(ctx, &RunOpts{Dir: tmpDir})
	require.NoError(t, err)
	_, _, err = NewCommand("-c", "user.name=a", "-c", "user.email=b", "commit", "-m=test").RunStdString(ctx, &RunOpts{Dir: tmpDir})
	require.NoError(t, err)
	submodules, err := GetTemplateSubmoduleCommits(DefaultContext, tmpDir)
	require.NoError(t, err)
	assert.Len(t, submodules, 1)
	assert.EqualValues(t, "new-dir", submodules[0].Path)
	assert.EqualValues(t, "1234567890123456789012345678901234567890", submodules[0].Commit)
}
