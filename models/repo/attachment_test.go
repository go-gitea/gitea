// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIncreaseDownloadCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	attachment, err := repo_model.GetAttachmentByUUID(t.Context(), "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), attachment.DownloadCount)

	// increase download count
	err = attachment.IncreaseDownloadCount(t.Context())
	assert.NoError(t, err)

	attachment, err = repo_model.GetAttachmentByUUID(t.Context(), "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), attachment.DownloadCount)
}

func TestGetByCommentOrIssueID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// count of attachments from issue ID
	attachments, err := repo_model.GetAttachmentsByIssueID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Len(t, attachments, 1)

	attachments, err = repo_model.GetAttachmentsByIssueID(t.Context(), 0)
	assert.NoError(t, err)
	assert.Empty(t, attachments)

	attachments, err = repo_model.GetAttachmentsByCommentID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Len(t, attachments, 2)
}

func TestDeleteAttachments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	count, err := repo_model.DeleteAttachmentsByIssue(t.Context(), 4, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	count, err = repo_model.DeleteAttachmentsByComment(t.Context(), 2, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	err = repo_model.DeleteAttachment(t.Context(), &repo_model.Attachment{ID: 8}, false)
	assert.NoError(t, err)

	attachment, err := repo_model.GetAttachmentByUUID(t.Context(), "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a18")
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrAttachmentNotExist(err))
	assert.Nil(t, attachment)
}

func TestGetAttachmentByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	attach, err := repo_model.GetAttachmentByID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attach.UUID)
}

func TestDeleteOrphanedAttachments(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	issue := &issues_model.Issue{}
	release := &repo_model.Release{}
	comment := &issues_model.Comment{}
	require.NoError(t, db.Insert(t.Context(), issue, release, comment))
	require.NoError(t, db.Insert(t.Context(), []*repo_model.Attachment{
		{UUID: "valid-issue", IssueID: issue.ID},
		{UUID: "valid-release", ReleaseID: release.ID},
		{UUID: "valid-comment", CommentID: comment.ID},
		{UUID: "unlinked"},
		{UUID: "orphaned-issue", IssueID: unittest.NonexistentID},
		{UUID: "orphaned-release", ReleaseID: unittest.NonexistentID},
		{UUID: "orphaned-comment", IssueID: issue.ID, CommentID: unittest.NonexistentID},
	}))

	require.NoError(t, repo_model.DeleteOrphanedAttachments(t.Context()))

	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{UUID: "valid-issue"})
	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{UUID: "valid-release"})
	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{UUID: "valid-comment"})
	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{UUID: "unlinked"})
	unittest.AssertNotExistsBean(t, &repo_model.Attachment{UUID: "orphaned-issue"})
	unittest.AssertNotExistsBean(t, &repo_model.Attachment{UUID: "orphaned-release"})
	unittest.AssertNotExistsBean(t, &repo_model.Attachment{UUID: "orphaned-comment"})
}

func TestAttachment_DownloadURL(t *testing.T) {
	attach := &repo_model.Attachment{
		UUID: "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
		ID:   1,
	}
	assert.Equal(t, "https://try.gitea.io/attachments/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attach.DownloadURL(t.Context()))
}

func TestUpdateAttachment(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	attach, err := repo_model.GetAttachmentByID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attach.UUID)

	attach.Name = "new_name"
	assert.NoError(t, repo_model.UpdateAttachment(t.Context(), attach))

	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{Name: "new_name"})
}

func TestGetAttachmentsByUUIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	attachList, err := repo_model.GetAttachmentsByUUIDs(t.Context(), []string{"a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a17", "not-existing-uuid"})
	assert.NoError(t, err)
	assert.Len(t, attachList, 2)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attachList[0].UUID)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a17", attachList[1].UUID)
	assert.Equal(t, int64(1), attachList[0].IssueID)
	assert.Equal(t, int64(5), attachList[1].IssueID)
}

func TestGetUnlinkedAttachmentsByUserID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	attachments, err := repo_model.GetUnlinkedAttachmentsByUserID(t.Context(), 8)
	assert.NoError(t, err)
	assert.Len(t, attachments, 1)
	assert.Equal(t, int64(10), attachments[0].ID)
	assert.Zero(t, attachments[0].IssueID)
	assert.Zero(t, attachments[0].ReleaseID)
	assert.Zero(t, attachments[0].CommentID)

	attachments, err = repo_model.GetUnlinkedAttachmentsByUserID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Empty(t, attachments)
}
