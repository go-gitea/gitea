// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitComment(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	const sha = "0123456789abcdef0123456789abcdef01234567"

	comment, err := issues_model.CreateCommitComment(t.Context(), &issues_model.CreateCommitCommentOptions{
		Doer:      doer,
		RepoID:    repo.ID,
		CommitSHA: sha,
		TreePath:  "README.md",
		Line:      3,
		Content:   "a commit comment",
	})
	require.NoError(t, err)
	assert.Equal(t, issues_model.CommentTypeCommitComment, comment.Type)
	assert.Equal(t, doer.ID, comment.PosterID)
	assert.Zero(t, comment.IssueID)
	assert.Equal(t, int64(3), comment.Line)
	assert.Equal(t, "README.md", comment.TreePath)

	comments, err := issues_model.FindCommitCommentsByCommitSHA(t.Context(), repo.ID, sha)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, comment.ID, comments[0].ID)
	require.NotNil(t, comments[0].Poster)
	assert.Equal(t, doer.ID, comments[0].Poster.ID)

	// a comment of another repository must not be returned for the same sha
	comments, err = issues_model.FindCommitCommentsByCommitSHA(t.Context(), repo.ID+1, sha)
	require.NoError(t, err)
	assert.Empty(t, comments)

	// loading by id also has to be scoped to the repository
	_, err = issues_model.GetCommitCommentByID(t.Context(), repo.ID, comment.ID)
	require.NoError(t, err)
	_, err = issues_model.GetCommitCommentByID(t.Context(), repo.ID+1, comment.ID)
	assert.Error(t, err)

	require.NoError(t, issues_model.DeleteCommitComment(t.Context(), comment))
	comments, err = issues_model.FindCommitCommentsByCommitSHA(t.Context(), repo.ID, sha)
	require.NoError(t, err)
	assert.Empty(t, comments)

	// deleting a comment has to remove the binding as well
	assert.Nil(t, unittest.GetBean(t, &issues_model.CommitComment{CommentID: comment.ID}))
}
