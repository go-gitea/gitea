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

func TestCommitCommentHashTag(t *testing.T) {
	c := &repo_model.CommitComment{ID: 42}
	assert.Equal(t, "commitcomment-42", c.HashTag())
}
