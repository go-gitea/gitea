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

	comment, err := issues_model.CreateCommitComment(t.Context(), &issues_model.CreateCommitCommentOptions{
		Repo:      repo,
		Doer:      doer,
		CommitSHA: "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		TreePath:  "README.md",
		Line:      1,
		Content:   "unit test commit comment",
		Patch:     "@@ -0,0 +1 @@\n+line",
	})
	require.NoError(t, err)
	require.NotNil(t, comment)
	assert.Equal(t, issues_model.CommentTypeCommitComment, comment.Type)
	assert.Equal(t, int64(0), comment.IssueID)
	assert.Equal(t, "unit test commit comment", comment.Content)

	unittest.AssertExistsAndLoadBean(t, &issues_model.CommitComment{
		RepoID:    repo.ID,
		CommitSHA: "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		CommentID: comment.ID,
	})

	found, err := issues_model.FindCommitCommentsByCommitSHA(t.Context(), repo.ID, "65f1bf27bc3bf70f64657658635e66094edbcb4d")
	require.NoError(t, err)
	assert.NotEmpty(t, found)

	byLine, err := issues_model.FindCommitCommentsByLine(t.Context(), repo.ID, "65f1bf27bc3bf70f64657658635e66094edbcb4d", "README.md", 1)
	require.NoError(t, err)
	require.Len(t, byLine, 1)
	assert.Equal(t, comment.ID, byLine[0].ID)

	// Wrong repo must not see the comment
	_, err = issues_model.GetCommitCommentByID(t.Context(), repo.ID+999, comment.ID)
	assert.Error(t, err)

	require.NoError(t, issues_model.DeleteCommitComment(t.Context(), repo.ID, comment.ID))
	unittest.AssertNotExistsBean(t, &issues_model.CommitComment{CommentID: comment.ID})
	unittest.AssertNotExistsBean(t, &issues_model.Comment{ID: comment.ID})
}

func TestCreateCommitCommentRejectsZeroLine(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	_, err := issues_model.CreateCommitComment(t.Context(), &issues_model.CreateCommitCommentOptions{
		Repo:      repo,
		Doer:      doer,
		CommitSHA: "abc",
		TreePath:  "a.go",
		Line:      0,
		Content:   "nope",
	})
	assert.ErrorIs(t, err, issues_model.ErrInvalidCommitCommentLine)
}


func TestUpdateCommitCommentAttachmentsBindsCommentID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	comment, err := issues_model.CreateCommitComment(t.Context(), &issues_model.CreateCommitCommentOptions{
		Repo:      repo,
		Doer:      doer,
		CommitSHA: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		TreePath:  "file.txt",
		Line:      2,
		Content:   "with attachment path",
	})
	require.NoError(t, err)

	a := &repo_model.Attachment{
		UUID:       "commit-comment-attach-uuid-1",
		RepoID:     repo.ID,
		UploaderID: doer.ID,
		Name:       "note.txt",
	}
	require.NoError(t, db.Insert(t.Context(), a))

	require.NoError(t, issues_model.UpdateCommitCommentAttachments(t.Context(), repo.ID, comment, []string{a.UUID}))

	reloaded := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{UUID: a.UUID})
	assert.Equal(t, comment.ID, reloaded.CommentID)
	assert.Equal(t, repo.ID, reloaded.RepoID)
	assert.Equal(t, int64(0), reloaded.IssueID)
	assert.Equal(t, int64(0), reloaded.ReleaseID)

	unlinked, err := repo_model.GetUnlinkedAttachmentsByUserID(t.Context(), doer.ID)
	require.NoError(t, err)
	for _, u := range unlinked {
		assert.NotEqual(t, a.UUID, u.UUID)
	}
}
