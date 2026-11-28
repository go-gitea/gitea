// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_GetCodeActivityStats(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepositoryLocal(t.Context(), bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	timeFrom, err := time.Parse(time.RFC3339, "2016-01-01T00:00:00+00:00")
	assert.NoError(t, err)

	code, err := bareRepo1.GetCodeActivityStats(t.Context(), timeFrom, "master")
	assert.NoError(t, err)
	assert.NotNil(t, code)

	assert.EqualValues(t, 7, code.CommitCount)
	assert.EqualValues(t, 3, code.AuthorCount)
	assert.EqualValues(t, 10, code.CommitCountInAllBranches)
	assert.EqualValues(t, 7, code.ChangedFiles)
	assert.Len(t, code.Authors, 3)

	authors, err := GetCodeActivityAuthors(t.Context(), bareRepo1, timeFrom)
	assert.NoError(t, err)
	assert.Len(t, authors, 3)
	assert.Equal(t, "tris.git@shoddynet.org", authors[1].Email)
	assert.EqualValues(t, 3, authors[1].Commits)
	assert.EqualValues(t, 5, authors[0].Commits)

	renameRepo, err := OpenRepositoryLocal(t.Context(), "../../tests/gitea-repositories-meta/user2/repo1.git")
	require.NoError(t, err)
	defer renameRepo.Close()
	code, err = renameRepo.GetCodeActivityStats(t.Context(), timeFrom, "sub-home-md-img-check")
	require.NoError(t, err)
	assert.EqualValues(t, 1, code.ChangedFiles)

	mergeRepo, err := OpenRepositoryLocal(t.Context(), filepath.Join(testReposDir, "repo6_merge"))
	require.NoError(t, err)
	defer mergeRepo.Close()
	code, err = mergeRepo.GetCodeActivityStats(t.Context(), time.Unix(1684066320, 0), "main")
	require.NoError(t, err)
	assert.Equal(t, &CodeActivityStats{CommitCountInAllBranches: 3, ChangedFiles: 3, Additions: 2, Deletions: 2, Authors: []*CodeActivityAuthor{}}, code)
}
