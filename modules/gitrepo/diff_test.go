// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/git/gitcmd"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDiffChangedFilesCountByCmdArgs(t *testing.T) {
	repoDir := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, gitcmd.NewCommand("init").AddDynamicArguments(repoDir).Run(t.Context()))
	require.NoError(t, gitcmd.NewCommand("config", "user.email", "user@example.com").WithDir(repoDir).Run(t.Context()))
	require.NoError(t, gitcmd.NewCommand("config", "user.name", "User").WithDir(repoDir).Run(t.Context()))

	writeFile := func(name, content string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(repoDir, name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, name), []byte(content), 0o644))
	}
	commit := func(message string) string {
		require.NoError(t, gitcmd.NewCommand("add", "-A").WithDir(repoDir).Run(t.Context()))
		require.NoError(t, gitcmd.NewCommand("commit", "-m").AddDynamicArguments(message).WithDir(repoDir).Run(t.Context()))
		stdout, _, err := gitcmd.NewCommand("rev-parse", "HEAD").WithDir(repoDir).RunStdString(t.Context())
		require.NoError(t, err)
		return strings.TrimSpace(stdout)
	}

	writeFile("delete me.txt", "remove\n")
	writeFile("dir/file with spaces.txt", "space old\n")
	writeFile("modify.txt", "old\n")
	baseCommitID := commit("base")

	require.NoError(t, os.Remove(filepath.Join(repoDir, "delete me.txt")))
	writeFile("added.txt", "added\n")
	writeFile("dir/file with spaces.txt", "space new\n")
	writeFile("modify.txt", "new\nextra\n")
	headCommitID := commit("change files")

	repo := &mockRepository{path: repoDir}

	count, err := GetDiffChangedFilesCountByCmdArgs(t.Context(), repo, nil, baseCommitID, baseCommitID)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	count, err = GetDiffChangedFilesCountByCmdArgs(t.Context(), repo, nil, baseCommitID, headCommitID)
	require.NoError(t, err)
	assert.Equal(t, 4, count)

	numFiles, additions, deletions, err := GetDiffShortStatByCmdArgs(t.Context(), repo, nil, baseCommitID, headCommitID)
	require.NoError(t, err)
	assert.Equal(t, 4, numFiles)
	assert.Equal(t, 4, additions)
	assert.Equal(t, 3, deletions)
}
