// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"github.com/stretchr/testify/assert"
)

func TestIncreaseDownloadCount(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

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
	assert.NoError(t, db.PrepareTestDatabase())

	// count of attachments from issue ID
	attachments, err := GetAttachmentsByIssueID(1)
	assert.NoError(t, err)
	assert.Len(t, attachments, 1)

	attachments, err = GetAttachmentsByCommentID(1)
	assert.NoError(t, err)
	assert.Len(t, attachments, 2)
}

func TestDeleteAttachments(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	count, err := DeleteAttachmentsByIssue(4, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

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
	assert.NoError(t, db.PrepareTestDatabase())

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
	assert.NoError(t, db.PrepareTestDatabase())

	attach, err := GetAttachmentByID(1)
	assert.NoError(t, err)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attach.UUID)

	attach.Name = "new_name"
	assert.NoError(t, UpdateAttachment(attach))

	db.AssertExistsAndLoadBean(t, &Attachment{Name: "new_name"})
}

func TestGetAttachmentsByUUIDs(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	attachList, err := GetAttachmentsByUUIDs(db.DefaultContext, []string{"a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a17", "not-existing-uuid"})
	assert.NoError(t, err)
	assert.Len(t, attachList, 2)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attachList[0].UUID)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a17", attachList[1].UUID)
	assert.Equal(t, int64(1), attachList[0].IssueID)
	assert.Equal(t, int64(5), attachList[1].IssueID)
}

func TestLinkedRepository(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())
	testCases := []struct {
		name             string
		attachID         int64
		expectedRepo     *Repository
		expectedUnitType UnitType
	}{
		{"LinkedIssue", 1, &Repository{ID: 1}, UnitTypeIssues},
		{"LinkedComment", 3, &Repository{ID: 1}, UnitTypePullRequests},
		{"LinkedRelease", 9, &Repository{ID: 1}, UnitTypeReleases},
		{"Notlinked", 10, nil, -1},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			attach, err := GetAttachmentByID(tc.attachID)
			assert.NoError(t, err)
			repo, unitType, err := attach.LinkedRepository()
			assert.NoError(t, err)
			if tc.expectedRepo != nil {
				assert.Equal(t, tc.expectedRepo.ID, repo.ID)
			}
			assert.Equal(t, tc.expectedUnitType, unitType)
		})
	}
}
