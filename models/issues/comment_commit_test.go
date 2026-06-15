// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCommitComment(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// An attachment that has been uploaded but is not yet bound to anything. Past
	// attempts at this feature lost such files to the orphaned-attachment cleanup.
	attachment := repo_model.Attachment{Name: "patch.txt", RepoID: repo.ID, UploaderID: doer.ID}
	require.NoError(t, db.Insert(t.Context(), &attachment))

	const sha = "1234567890123456789012345678901234567890"
	comment, err := issues_model.CreateCommitComment(t.Context(), &issues_model.CreateCommitCommentOptions{
		Doer:        doer,
		Repo:        repo,
		CommitSHA:   sha,
		Content:     "looks good",
		TreePath:    "README.md",
		LineNum:     4,
		Attachments: []string{attachment.UUID},
	})
	require.NoError(t, err)

	assert.Equal(t, issues_model.CommentTypeCode, comment.Type)
	assert.Equal(t, int64(0), comment.IssueID)
	assert.Equal(t, repo.ID, comment.RepoID)
	assert.Equal(t, sha, comment.CommitSHA)
	assert.Equal(t, int64(4), comment.Line)
	unittest.AssertExistsAndLoadBean(t, comment) // assert actually added to DB

	// The attachment must be bound to the comment, otherwise it would be removed as
	// orphaned. Verify both the binding and that it is no longer considered unlinked.
	bound := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: attachment.ID})
	assert.Equal(t, comment.ID, bound.CommentID)

	unlinked, err := repo_model.GetUnlinkedAttachmentsByUserID(t.Context(), doer.ID)
	require.NoError(t, err)
	for _, a := range unlinked {
		assert.NotEqual(t, attachment.ID, a.ID, "bound commit-comment attachment must not be considered unlinked")
	}
}

func TestFetchCommitCodeComments(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	const sha = "abcdefabcdefabcdefabcdefabcdefabcdefabcd"

	_, err := issues_model.CreateCommitComment(t.Context(), &issues_model.CreateCommitCommentOptions{
		Doer: doer, Repo: repo, CommitSHA: sha, Content: "on the new side", TreePath: "a.txt", LineNum: 3,
	})
	require.NoError(t, err)
	_, err = issues_model.CreateCommitComment(t.Context(), &issues_model.CreateCommitCommentOptions{
		Doer: doer, Repo: repo, CommitSHA: sha, Content: "on the old side", TreePath: "a.txt", LineNum: -2,
	})
	require.NoError(t, err)

	res, err := issues_model.FetchCommitCodeComments(t.Context(), repo, sha, doer)
	require.NoError(t, err)
	require.Contains(t, res, "a.txt")
	assert.Len(t, res["a.txt"][3], 1)
	assert.Len(t, res["a.txt"][-2], 1)
	assert.Equal(t, "on the new side", res["a.txt"][3][0].Content)
	assert.NotEmpty(t, res["a.txt"][3][0].RenderedContent, "comment content should be rendered")

	count, err := issues_model.CountCommitComments(t.Context(), repo, sha)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Comments must be scoped to the repository: a fork sharing the same commit SHA
	// must not see another repository's commit comments.
	other := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	resOther, err := issues_model.FetchCommitCodeComments(t.Context(), other, sha, doer)
	require.NoError(t, err)
	assert.Empty(t, resOther)
}
