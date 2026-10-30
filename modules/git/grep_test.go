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
	repo, err := OpenRepositoryLocal(t.Context(), filepath.Join(testReposDir, "language_stats_repo"))
	assert.NoError(t, err)
	defer repo.Close()

	expectedResults := []*GrepResult{
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
	}
	for _, testcase := range []struct {
		options  GrepOptions
		expected []*GrepResult
	}{
		{GrepOptions{}, expectedResults},
		{GrepOptions{PathspecList: []string{":(glob)java-hello/*"}}, expectedResults[:1]},
		{GrepOptions{PathspecList: []string{":(glob,exclude)java-hello/*"}}, expectedResults[1:]},
		{GrepOptions{MaxResultLimit: 1}, expectedResults[:1]},
		{GrepOptions{MaxResultLimit: 1, MaxLineLength: 39}, []*GrepResult{
			{
				Filename:    "java-hello/main.java",
				LineNumbers: []int{3},
				LineCodes:   []string{" public static void main(String[] arg"},
			},
		}},
	} {
		res, err := GrepSearch(t.Context(), repo, "void", testcase.options)
		assert.NoError(t, err)
		assert.Equal(t, testcase.expected, res)
	}

	res, err := GrepSearch(t.Context(), repo, "no-such-content", GrepOptions{})
	assert.NoError(t, err)
	assert.Empty(t, res)

	// search a string starting with a dash
	res, err = GrepSearch(t.Context(), repo, "-v", GrepOptions{})
	assert.NoError(t, err)
	assert.Equal(t, []*GrepResult{{Filename: ".gitattributes", LineNumbers: []int{1}, LineCodes: []string{"*.vendor.java linguist-vendored"}}}, res)

	nonExistingRepo := &Repository{RepositoryBase: RepositoryBase{repoFacade: gitrepo.RepositoryUnmanaged("no-such-git-repo")}}
	res, err = GrepSearch(t.Context(), nonExistingRepo, "no-such-content", GrepOptions{})
	assert.Error(t, err)
	assert.Empty(t, res)
}
