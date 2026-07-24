// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestGrepSearch(t *testing.T) {
	defer test.MockVariableValue(&setting.RepoRootPath, t.TempDir())()
	repo, err := OpenRepositoryLocal(filepath.Join(testReposDir, "language_stats_repo"))
	assert.NoError(t, err)
	defer repo.Close()

	res, err := GrepSearch(t.Context(), repo, "void", GrepOptions{})
	assert.NoError(t, err)
	assert.Equal(t, []*GrepResult{
		{
			Filename:    "java-hello/main.java",
			LineNumbers: []int{3},
			LineCodes:   []string{" public static void main(String[] args)"},
		},
		{
			Filename:    "main.vendor.java",
			LineNumbers: []int{3},
			LineCodes:   []string{" public static void main(String[] args)"},
		},
	}, res)

	res, err = GrepSearch(t.Context(), repo, "void", GrepOptions{PathspecList: []string{":(glob)java-hello/*"}})
	assert.NoError(t, err)
	assert.Equal(t, []*GrepResult{
		{
			Filename:    "java-hello/main.java",
			LineNumbers: []int{3},
			LineCodes:   []string{" public static void main(String[] args)"},
		},
	}, res)

	res, err = GrepSearch(t.Context(), repo, "void", GrepOptions{PathspecList: []string{":(glob,exclude)java-hello/*"}})
	assert.NoError(t, err)
	assert.Equal(t, []*GrepResult{
		{
			Filename:    "main.vendor.java",
			LineNumbers: []int{3},
			LineCodes:   []string{" public static void main(String[] args)"},
		},
	}, res)

	res, err = GrepSearch(t.Context(), repo, "void", GrepOptions{MaxResultLimit: 1})
	assert.NoError(t, err)
	assert.Equal(t, []*GrepResult{
		{
			Filename:    "java-hello/main.java",
			LineNumbers: []int{3},
			LineCodes:   []string{" public static void main(String[] args)"},
		},
	}, res)

	res, err = GrepSearch(t.Context(), repo, "void", GrepOptions{MaxResultLimit: 1, MaxLineLength: 39})
	assert.NoError(t, err)
	assert.Equal(t, []*GrepResult{
		{
			Filename:    "java-hello/main.java",
			LineNumbers: []int{3},
			LineCodes:   []string{" public static void main(String[] arg"},
		},
	}, res)

	res, err = GrepSearch(t.Context(), repo, "no-such-content", GrepOptions{})
	assert.NoError(t, err)
	assert.Empty(t, res)

	nonExistingRepo := &Repository{RepositoryBase: RepositoryBase{repoFacade: gitrepo.RepositoryUnmanaged("no-such-git-repo")}}
	res, err = GrepSearch(t.Context(), nonExistingRepo, "no-such-content", GrepOptions{})
	assert.Error(t, err)
	assert.Empty(t, res)
}
