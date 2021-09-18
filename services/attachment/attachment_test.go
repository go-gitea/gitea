// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package attachment

import (
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", ".."))
}

func TestUploadAttachment(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 1}).(*models.User)

	fPath := "./attachment_test.go"
	f, err := os.Open(fPath)
	assert.NoError(t, err)
	defer f.Close()

	attach, err := NewAttachment(&models.Attachment{
		RepoID:     1,
		UploaderID: user.ID,
		Name:       filepath.Base(fPath),
	}, f)
	assert.NoError(t, err)

	attachment, err := models.GetAttachmentByUUID(attach.UUID)
	assert.NoError(t, err)
	assert.EqualValues(t, user.ID, attachment.UploaderID)
	assert.Equal(t, int64(0), attachment.DownloadCount)
}
