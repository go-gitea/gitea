// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package attachment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/storage"

	_ "code.gitea.io/gitea/models/actions"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestUploadAttachment(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	fPath := "./attachment_test.go"
	f, err := os.Open(fPath)
	assert.NoError(t, err)
	defer f.Close()

	attach, err := NewAttachment(db.DefaultContext, &repo_model.Attachment{
		RepoID:     1,
		UploaderID: user.ID,
		Name:       filepath.Base(fPath),
	}, f, -1)
	assert.NoError(t, err)

	attachment, err := repo_model.GetAttachmentByUUID(db.DefaultContext, attach.UUID)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, attachment.UploaderID)
	assert.Equal(t, int64(0), attachment.DownloadCount)
}

func TestDeleteAttachments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	attachment8 := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 8})
	const attachment8Content = "test content for attachment 8" // 29 bytes
	_, err := storage.Attachments.Save(attachment8.RelativePath(), strings.NewReader(attachment8Content), int64(len(attachment8Content)))
	assert.NoError(t, err)

	fileInfo, err := storage.Attachments.Stat(attachment8.RelativePath())
	assert.NoError(t, err)
	assert.Equal(t, attachment8.Size, fileInfo.Size())

	err = DeleteAttachment(db.DefaultContext, attachment8)
	assert.NoError(t, err)

	attachment, err := repo_model.GetAttachmentByUUID(db.DefaultContext, attachment8.UUID)
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrAttachmentNotExist(err))
	assert.Nil(t, attachment)

	// allow the queue to process the deletion
	time.Sleep(1 * time.Second)

	_, err = storage.Attachments.Stat(attachment8.RelativePath())
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}
