// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGrepSearch(t *testing.T) {
	repo, err := openRepositoryWithDefaultContext(filepath.Join(testReposDir, "language_stats_repo"))
	assert.NoError(t, err)
	defer repo.Close()

	res, err := GrepSearch(context.Background(), repo, "void", GrepOptions{})
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

	res, err = GrepSearch(context.Background(), repo, "void", GrepOptions{PathspecList: []string{":(glob)java-hello/*"}})
	assert.NoError(t, err)
	assert.Equal(t, []*GrepResult{
		{
			Filename:    "java-hello/main.java",
			LineNumbers: []int{3},
			LineCodes:   []string{" public static void main(String[] args)"},
		},
	}, res)

	res, err = GrepSearch(context.Background(), repo, "void", GrepOptions{PathspecList: []string{":(glob,exclude)java-hello/*"}})
	assert.NoError(t, err)
	assert.Equal(t, []*GrepResult{
		{
			Filename:    "main.vendor.java",
			LineNumbers: []int{3},
			LineCodes:   []string{" public static void main(String[] args)"},
		},
	}, res)

	res, err = GrepSearch(context.Background(), repo, "void", GrepOptions{MaxResultLimit: 1})
	assert.NoError(t, err)
	assert.Equal(t, []*GrepResult{
		{
			Filename:    "java-hello/main.java",
			LineNumbers: []int{3},
			LineCodes:   []string{" public static void main(String[] args)"},
		},
	}, res)

	res, err = GrepSearch(context.Background(), repo, "void", GrepOptions{MaxResultLimit: 1, MaxLineLength: 39})
	assert.NoError(t, err)
	assert.Equal(t, []*GrepResult{
		{
			Filename:    "java-hello/main.java",
			LineNumbers: []int{3},
			LineCodes:   []string{" public static void main(String[] arg"},
		},
	}, res)

	res, err = GrepSearch(context.Background(), repo, "no-such-content", GrepOptions{})
	assert.NoError(t, err)
	assert.Empty(t, res)

	res, err = GrepSearch(context.Background(), &Repository{Path: "no-such-git-repo"}, "no-such-content", GrepOptions{})
	assert.Error(t, err)
	assert.Empty(t, res)
}
