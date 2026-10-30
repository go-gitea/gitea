// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitComment(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	const sha, otherSHA = "sha", "other"

	comment := &issues_model.Comment{PosterID: 2, TreePath: "README.md", Line: 1}
	require.NoError(t, issues_model.CreateCommitComment(t.Context(), repo.ID, sha, comment))
	require.NoError(t, issues_model.CreateCommitComment(t.Context(), repo.ID, otherSHA, &issues_model.Comment{}))
	require.NoError(t, issues_model.CreateCommitComment(t.Context(), 2, sha, &issues_model.Comment{}))

	comments, err := issues_model.FetchCommitCodeComments(t.Context(), repo, sha)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	require.Len(t, comments["README.md"][1], 1)
	assert.Equal(t, comment.ID, comments["README.md"][1][0].ID)

	_, err = issues_model.GetCommitCommentByID(t.Context(), 2, sha, comment.ID)
	assert.True(t, issues_model.IsErrCommentNotExist(err))
	_, err = issues_model.GetCommitCommentByID(t.Context(), repo.ID, otherSHA, comment.ID)
	assert.True(t, issues_model.IsErrCommentNotExist(err))

	comment, err = issues_model.GetCommitCommentByID(t.Context(), repo.ID, sha, comment.ID)
	require.NoError(t, err)
	require.NoError(t, issues_model.DeleteComment(t.Context(), comment))
	unittest.AssertNotExistsBean(t, &issues_model.Comment{ID: comment.ID})
	unittest.AssertCount(t, &issues_model.CommitComment{CommentID: comment.ID}, 0)

	require.NoError(t, issues_model.DeleteCommitCommentsByRepoID(t.Context(), repo.ID))
	unittest.AssertCount(t, &issues_model.Comment{Type: issues_model.CommentTypeCommitComment}, 1)
	unittest.AssertCount(t, &issues_model.CommitComment{RepoID: repo.ID}, 0)
	unittest.AssertCount(t, &issues_model.CommitComment{RepoID: 2}, 1)
}
