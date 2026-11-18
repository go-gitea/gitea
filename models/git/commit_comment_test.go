// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git_test

import (
	"testing"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitComment(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	newAttachment := func(uuid string, repoID int64) *repo_model.Attachment {
		a := &repo_model.Attachment{UUID: uuid, RepoID: repoID, Name: uuid}
		require.NoError(t, db.Insert(ctx, a))
		return a
	}

	c := &git_model.CommitComment{RepoID: 1, CommitSHA: "sha1", PosterID: 2, TreePath: "a.txt", Line: -3, Content: "hello"}
	require.NoError(t, git_model.CreateCommitComment(ctx, c, []string{newAttachment("cc-keep", 1).UUID, newAttachment("cc-drop", 1).UUID}))
	assert.Equal(t, "previous", c.DiffSide())
	assert.Equal(t, int64(3), c.UnsignedLine())
	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{UUID: "cc-keep", CommitCommentID: c.ID})

	other := &git_model.CommitComment{RepoID: 1, CommitSHA: "sha2", PosterID: 2, TreePath: "a.txt", Line: 1, Content: "other"}
	require.NoError(t, git_model.CreateCommitComment(ctx, other, nil))
	list, err := git_model.ListCommitComments(ctx, 1, "sha1")
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.NoError(t, git_model.LoadCommitCommentsAttachments(ctx, list))
	assert.Len(t, list[0].Attachments, 2)

	t.Run("AttachmentOfOtherRepoOrAlreadyLinked", func(t *testing.T) {
		bad := &git_model.CommitComment{RepoID: 1, CommitSHA: "sha1", PosterID: 2, TreePath: "a.txt", Line: 1, Content: "x"}
		assert.Error(t, git_model.CreateCommitComment(ctx, bad, []string{newAttachment("cc-other-repo", 2).UUID}))
		unittest.AssertNotExistsBean(t, &git_model.CommitComment{Content: "x"}) // rolled back

		bad = &git_model.CommitComment{RepoID: 1, CommitSHA: "sha1", PosterID: 2, TreePath: "a.txt", Line: 1, Content: "y"}
		assert.Error(t, git_model.CreateCommitComment(ctx, bad, []string{"cc-keep"}))
	})

	t.Run("Update", func(t *testing.T) {
		loaded, err := git_model.GetCommitCommentByID(ctx, c.ID)
		require.NoError(t, err)
		loaded.Content = "changed"
		require.NoError(t, git_model.UpdateCommitComment(ctx, loaded, 0, []string{"cc-keep"}, true))
		unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{UUID: "cc-keep"})
		unittest.AssertNotExistsBean(t, &repo_model.Attachment{UUID: "cc-drop"})
		assert.Len(t, loaded.Attachments, 1)

		loaded.Content = "stale"
		assert.ErrorIs(t, git_model.UpdateCommitComment(ctx, loaded, 0, nil, false), git_model.ErrCommitCommentAlreadyChanged)
		loaded, err = git_model.GetCommitCommentByID(ctx, c.ID)
		require.NoError(t, err)
		assert.Equal(t, "changed", loaded.Content)
		assert.Equal(t, 1, loaded.ContentVersion)
	})

	t.Run("Delete", func(t *testing.T) {
		require.NoError(t, git_model.DeleteCommitComment(ctx, c))
		unittest.AssertNotExistsBean(t, &git_model.CommitComment{ID: c.ID})
		unittest.AssertNotExistsBean(t, &repo_model.Attachment{UUID: "cc-keep"})
		unittest.AssertExistsAndLoadBean(t, &git_model.CommitComment{ID: other.ID})

		_, err := git_model.GetCommitCommentByID(ctx, c.ID)
		assert.ErrorIs(t, err, git_model.ErrCommitCommentNotExist)
	})
}
