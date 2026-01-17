// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestMigrate_InsertReleases(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	a := &Attachment{
		UUID: "a0eebc91-9c0c-4ef7-bb6e-6bb9bd380a12",
	}
	r := &Release{
		Attachments: []*Attachment{a},
	}

	err := InsertReleases(t.Context(), r)
	assert.NoError(t, err)
}

func Test_FindTagsByCommitIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	sha1Rels, err := FindTagsByCommitIDs(t.Context(), 1, "65f1bf27bc3bf70f64657658635e66094edbcb4d")
	assert.NoError(t, err)
	assert.Len(t, sha1Rels, 1)
	rels := sha1Rels["65f1bf27bc3bf70f64657658635e66094edbcb4d"]
	assert.Len(t, rels, 3)
	assert.Equal(t, "v1.1", rels[0].TagName)
	assert.Equal(t, "delete-tag", rels[1].TagName)
	assert.Equal(t, "v1.0", rels[2].TagName)
}

func TestAddReleaseAttachmentsRejectsDifferentRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	uuid := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12" // attachment 2 belongs to repo 2
	err := AddReleaseAttachments(t.Context(), 1, []string{uuid})
	assert.Error(t, err)
	assert.ErrorIs(t, err, util.ErrPermissionDenied)

	attach, err := GetAttachmentByUUID(t.Context(), uuid)
	assert.NoError(t, err)
	assert.Zero(t, attach.ReleaseID, "attachment should not be linked to release on failure")
}
