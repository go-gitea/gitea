// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIncreaseDownloadCount(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	attachment, err := GetAttachmentByUUID("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), attachment.DownloadCount)

	// increase download count
	err = attachment.IncreaseDownloadCount()
	assert.NoError(t, err)

	attachment, err = GetAttachmentByUUID("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), attachment.DownloadCount)
}

func TestGetByCommentOrIssueID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// count of attachments from issue ID
	attachments, err := GetAttachmentsByIssueID(1)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(attachments))

	attachments, err = GetAttachmentsByCommentID(1)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(attachments))
}

func TestDeleteAttachments(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	count, err := DeleteAttachmentsByIssue(4, false)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	count, err = DeleteAttachmentsByComment(2, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	err = DeleteAttachment(&Attachment{ID: 8}, false)
	assert.NoError(t, err)

	attachment, err := GetAttachmentByUUID("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a18")
	assert.Error(t, err)
	assert.True(t, IsErrAttachmentNotExist(err))
	assert.Nil(t, attachment)
}

func TestGetAttachmentByID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	attach, err := GetAttachmentByID(1)
	assert.NoError(t, err)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attach.UUID)
}

func TestAttachment_DownloadURL(t *testing.T) {
	attach := &Attachment{
		UUID: "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
		ID:   1,
	}
	assert.Equal(t, "https://try.gitea.io/attachments/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attach.DownloadURL())
}

func TestUpdateAttachment(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	attach, err := GetAttachmentByID(1)
	assert.NoError(t, err)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attach.UUID)

	attach.Name = "new_name"
	assert.NoError(t, UpdateAttachment(attach))

	AssertExistsAndLoadBean(t, &Attachment{Name: "new_name"})
}
