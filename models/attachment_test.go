// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUploadAttachment(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)

	var fPath = "./attachment_test.go"
	f, err := os.Open(fPath)
	assert.NoError(t, err)
	defer f.Close()

	var buf = make([]byte, 1024)
	n, err := f.Read(buf)
	assert.NoError(t, err)
	buf = buf[:n]

	attach, err := NewAttachment(&Attachment{
		UploaderID: user.ID,
		Name:       filepath.Base(fPath),
	}, buf, f)
	assert.NoError(t, err)

	attachment, err := GetAttachmentByUUID(attach.UUID)
	assert.NoError(t, err)
	assert.EqualValues(t, user.ID, attachment.UploaderID)
	assert.Equal(t, int64(0), attachment.DownloadCount)
}

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
	assert.Equal(t, 1, len(attachments))

	attachments, err = GetAttachmentsByCommentID(1)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(attachments))
}

func TestDeleteAttachments(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

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

func TestGetAttachmentsByUUIDs(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	attachList, err := GetAttachmentsByUUIDs([]string{"a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a17", "not-existing-uuid"})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(attachList))
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attachList[0].UUID)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a17", attachList[1].UUID)
	assert.Equal(t, int64(1), attachList[0].IssueID)
	assert.Equal(t, int64(5), attachList[1].IssueID)
}

func TestLinkedRepository(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
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
