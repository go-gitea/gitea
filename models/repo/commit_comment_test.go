// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCommitCommentRejectsZeroLine(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	c := &repo_model.CommitComment{
		RepoID:    1,
		CommitSHA: "0000000000000000000000000000000000000000",
		TreePath:  "README.md",
		Line:      0,
		PosterID:  2,
		Content:   "test",
	}
	err := repo_model.CreateCommitComment(t.Context(), c)
	require.Error(t, err)
	assert.ErrorIs(t, err, repo_model.ErrInvalidCommitCommentLine)
}

func TestCommitCommentDiffSideAndUnsignedLine(t *testing.T) {
	left := &repo_model.CommitComment{Line: -7}
	right := &repo_model.CommitComment{Line: 12}

	assert.Equal(t, "previous", left.DiffSide())
	assert.Equal(t, "proposed", right.DiffSide())
	assert.EqualValues(t, 7, left.UnsignedLine())
	assert.EqualValues(t, 12, right.UnsignedLine())
}

func TestFindCommitCommentsByLineAndPosters(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	const sha = "1111111111111111111111111111111111111111"
	for _, c := range []*repo_model.CommitComment{
		{RepoID: 1, CommitSHA: sha, TreePath: "a.go", Line: 3, PosterID: 2, Content: "first"},
		{RepoID: 1, CommitSHA: sha, TreePath: "a.go", Line: 3, PosterID: 4, Content: "second"},
		{RepoID: 1, CommitSHA: sha, TreePath: "a.go", Line: -3, PosterID: 2, Content: "other side"},
		{RepoID: 1, CommitSHA: sha, TreePath: "b.go", Line: 3, PosterID: 2, Content: "other file"},
	} {
		require.NoError(t, repo_model.CreateCommitComment(t.Context(), c))
	}

	comments, err := repo_model.FindCommitCommentsByLine(t.Context(), 1, sha, "a.go", 3)
	require.NoError(t, err)
	require.Len(t, comments, 2)
	assert.Equal(t, "first", comments[0].Content)
	assert.Equal(t, "second", comments[1].Content)
	assert.NotNil(t, comments[0].Poster)

	posterIDs, err := repo_model.GetCommitCommentPosterIDs(t.Context(), 1, sha)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{2, 4}, posterIDs)

	file, err := repo_model.FindCommitCommentsForFile(t.Context(), 1, sha, "a.go")
	require.NoError(t, err)
	require.NotNil(t, file)
	assert.Len(t, file.Right[3], 2)
	assert.Len(t, file.Left[3], 1)

	missing, err := repo_model.FindCommitCommentsForFile(t.Context(), 1, sha, "c.go")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestCommitCommentHashTag(t *testing.T) {
	c := &repo_model.CommitComment{ID: 42}
	assert.Equal(t, "commitcomment-42", c.HashTag())
}
