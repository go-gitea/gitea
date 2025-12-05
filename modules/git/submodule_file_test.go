// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmoduleFile(t *testing.T) {
	assert.Nil(t, (*SubmoduleFile)(nil).SubmoduleWebLinkTree(t.Context()))
	assert.Nil(t, (*SubmoduleFile)(nil).SubmoduleWebLinkCompare(t.Context(), "", ""))
	assert.Nil(t, (&SubmoduleFile{}).SubmoduleWebLinkTree(t.Context()))
	assert.Nil(t, (&SubmoduleFile{}).SubmoduleWebLinkCompare(t.Context(), "", ""))

	t.Run("GitHubRepo", func(t *testing.T) {
		sf := NewSubmoduleFile("/any/repo-link", "full-path", "git@github.com:user/repo.git", "aaaa")
		wl := sf.SubmoduleWebLinkTree(t.Context())
		assert.Equal(t, "https://github.com/user/repo", wl.RepoWebLink)
		assert.Equal(t, "https://github.com/user/repo/tree/aaaa", wl.CommitWebLink)

		wl = sf.SubmoduleWebLinkCompare(t.Context(), "1111", "2222")
		assert.Equal(t, "https://github.com/user/repo", wl.RepoWebLink)
		assert.Equal(t, "https://github.com/user/repo/compare/1111...2222", wl.CommitWebLink)
	})

	t.Run("RelativePath", func(t *testing.T) {
		sf := NewSubmoduleFile("/subpath/any/repo-home-link", "full-path", "../../user/repo", "aaaa")
		wl := sf.SubmoduleWebLinkTree(t.Context())
		assert.Equal(t, "/subpath/user/repo", wl.RepoWebLink)
		assert.Equal(t, "/subpath/user/repo/tree/aaaa", wl.CommitWebLink)

		sf = NewSubmoduleFile("/subpath/any/repo-home-link", "dir/submodule", "../../user/repo", "aaaa")
		wl = sf.SubmoduleWebLinkCompare(t.Context(), "1111", "2222")
		assert.Equal(t, "/subpath/user/repo", wl.RepoWebLink)
		assert.Equal(t, "/subpath/user/repo/compare/1111...2222", wl.CommitWebLink)
	})
}

func TestGetRepoSubmoduleFiles(t *testing.T) {
	testRepoPath := filepath.Join(testReposDir, "repo4_submodules")
	submodules, err := GetRepoSubmoduleFiles(t.Context(), testRepoPath, "HEAD")
	require.NoError(t, err)

	assert.Len(t, submodules, 2)

	assert.Equal(t, "<°)))><", submodules[0].FullPath())
	assert.Equal(t, "d2932de67963f23d43e1c7ecf20173e92ee6c43c", submodules[0].RefID())

	assert.Equal(t, "libtest", submodules[1].FullPath())
	assert.Equal(t, "1234567890123456789012345678901234567890", submodules[1].RefID())
}

func TestAddTemplateSubmoduleIndexes(t *testing.T) {
	ctx := t.Context()
	tmpDir := t.TempDir()
	var err error
	_, _, err = gitcmd.NewCommand("init").WithDir(tmpDir).RunStdString(ctx)
	require.NoError(t, err)
	_ = os.Mkdir(filepath.Join(tmpDir, "new-dir"), 0o755)
	err = AddSubmodulesToRepoIndex(ctx, tmpDir, []SubmoduleFile{{fullPath: "new-dir", refID: "1234567890123456789012345678901234567890"}})
	require.NoError(t, err)
	_, _, err = gitcmd.NewCommand("add", "--all").WithDir(tmpDir).RunStdString(ctx)
	require.NoError(t, err)
	_, _, err = gitcmd.NewCommand("-c", "user.name=a", "-c", "user.email=b", "commit", "-m=test").WithDir(tmpDir).RunStdString(ctx)
	require.NoError(t, err)
	submodules, err := GetRepoSubmoduleFiles(t.Context(), tmpDir, "HEAD")
	require.NoError(t, err)
	assert.Len(t, submodules, 1)
	assert.Equal(t, "new-dir", submodules[0].FullPath())
	assert.Equal(t, "1234567890123456789012345678901234567890", submodules[0].RefID())
}
