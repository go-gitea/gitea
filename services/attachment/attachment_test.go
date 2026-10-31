// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package attachment

import (
	"strings"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/base"

	_ "gitea.dev/models/actions"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestUploadAttachment(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	content := "attachment content"
	attach, err := NewAttachment(t.Context(), &repo_model.Attachment{
		RepoID:     1,
		UploaderID: user.ID,
		Name:       "attachment.txt",
	}, strings.NewReader(content), -1)
	assert.NoError(t, err)

	attachment, err := repo_model.GetAttachmentByUUID(t.Context(), attach.UUID)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, attachment.UploaderID)
	assert.Equal(t, int64(0), attachment.DownloadCount)
	assert.Equal(t, base.EncodeSha256(content), attachment.HashSHA256)
}
