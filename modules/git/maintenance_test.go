// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitea.dev/modules/git/gitcmd"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNeedsGarbageCollection(t *testing.T) {
	if !DefaultFeatures().CheckVersionAtLeast("2.41") {
		t.Skip("git gc writes cruft packs by default since git 2.41")
	}
	ctx := t.Context()
	repoPath := filepath.Join(t.TempDir(), "repo.git")
	require.NoError(t, gitcmd.NewCommand("init", "--bare").AddDynamicArguments(repoPath).Run(ctx))
	repo := mockRepository(repoPath)
	assert.False(t, NeedsGarbageCollection(mockRepository(filepath.Join(repoPath, "missing.git"))))
	run := func(cmd *gitcmd.Command, stdin string) string {
		stdout, _, err := cmd.WithDir(repoPath).WithStdinBytes([]byte(stdin)).RunStdString(ctx)
		require.NoError(t, err)
		return strings.TrimSpace(stdout)
	}
	for _, branch := range []string{"a", "b"} {
		blobID := run(gitcmd.NewCommand("hash-object", "-w", "--stdin"), branch)
		treeID := run(gitcmd.NewCommand("mktree"), "100644 blob "+blobID+"\tfile\n")
		commitID := run(gitcmd.NewCommand("commit-tree").AddDynamicArguments(treeID).AddOptionValues("-m", branch), "")
		run(gitcmd.NewCommand("update-ref").AddDynamicArguments("refs/heads/"+branch, commitID), "")
		run(gitcmd.NewCommand("repack", "-d"), "")
	}
	assert.True(t, NeedsGarbageCollection(repo))

	run(gitcmd.NewCommand("update-ref", "-d", "refs/heads/b"), "")
	run(gitcmd.NewCommand("gc"), "")
	cruftPacks, _ := filepath.Glob(filepath.Join(repoPath, "objects", "pack", "*.mtimes"))
	assert.Len(t, cruftPacks, 1)
	assert.False(t, NeedsGarbageCollection(repo))

	keptPack := run(gitcmd.NewCommand("pack-objects", "objects/pack/pack"), run(gitcmd.NewCommand("hash-object", "-w", "--stdin"), "kept")+"\n")
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "objects", "pack", "pack-"+keptPack+".keep"), nil, 0o644))
	run(gitcmd.NewCommand("prune-packed"), "")
	assert.False(t, NeedsGarbageCollection(repo))

	run(gitcmd.NewCommand("hash-object", "-w", "--stdin"), "loose")
	assert.True(t, NeedsGarbageCollection(repo))
}
