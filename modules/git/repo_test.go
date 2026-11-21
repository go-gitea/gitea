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

func TestPushMirrorExclude(t *testing.T) {
	ctx := t.Context()
	srcPath := filepath.Join(t.TempDir(), "src.git")
	dstPath := filepath.Join(t.TempDir(), "dst.git")
	require.NoError(t, Clone(ctx, filepath.Join(testReposDir, "repo1_bare"), srcPath, CloneRepoOptions{Bare: true}))
	require.NoError(t, InitRepositoryLocal(ctx, dstPath, true, Sha1ObjectFormat.Name(), "master"))

	src, err := OpenRepositoryLocal(ctx, srcPath)
	require.NoError(t, err)
	defer src.Close()
	headID, err := src.GetRefCommitID(ctx, "refs/heads/master")
	require.NoError(t, err)

	run := func(dir string, cmd *gitcmd.Command) {
		_, _, err := cmd.WithDir(dir).RunStdString(ctx)
		require.NoError(t, err)
	}
	run(srcPath, gitcmd.NewCommand("update-ref", "refs/excluded/x").AddDynamicArguments(headID))
	run(srcPath, gitcmd.NewCommand("remote", "add", "--mirror=push", "mirror").AddDynamicArguments(dstPath))

	opts := PushOptions{Remote: "mirror", Force: true, Mirror: true, MirrorExclude: []string{"refs/excluded/x"}}
	require.NoError(t, Push(ctx, srcPath, opts))
	run(dstPath, gitcmd.NewCommand("update-ref", "refs/heads/stale").AddDynamicArguments(headID))
	require.NoError(t, Push(ctx, srcPath, opts))

	dst, err := OpenRepositoryLocal(ctx, dstPath)
	require.NoError(t, err)
	defer dst.Close()
	srcRefs, err := src.GetRefsFiltered(ctx, "refs/heads/")
	require.NoError(t, err)
	dstRefs, err := dst.GetRefs(ctx)
	require.NoError(t, err)
	var srcNames, dstNames []string
	for _, ref := range srcRefs {
		srcNames = append(srcNames, ref.Name)
	}
	for _, ref := range dstRefs {
		dstNames = append(dstNames, ref.Name)
	}
	assert.Subset(t, dstNames, srcNames)
	assert.NotContains(t, dstNames, "refs/excluded/x")
	assert.NotContains(t, dstNames, "refs/heads/stale")
}
