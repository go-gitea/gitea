// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/avatar"

	"github.com/stretchr/testify/assert"
)

func TestUploadAvatar(t *testing.T) {
	// Generate image
	myImage := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buff bytes.Buffer
	png.Encode(&buff, myImage)

	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})

	err := UploadAvatar(db.DefaultContext, repo, buff.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, avatar.HashAvatar(10, buff.Bytes()), repo.Avatar)
}

func TestUploadBigAvatar(t *testing.T) {
	// Generate BIG image
	myImage := image.NewRGBA(image.Rect(0, 0, 5000, 1))
	var buff bytes.Buffer
	png.Encode(&buff, myImage)

	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})

	err := UploadAvatar(db.DefaultContext, repo, buff.Bytes())
	assert.Error(t, err)
}

func TestDeleteAvatar(t *testing.T) {
	// Generate image
	myImage := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buff bytes.Buffer
	png.Encode(&buff, myImage)

	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})

	err := UploadAvatar(db.DefaultContext, repo, buff.Bytes())
	assert.NoError(t, err)

	err = DeleteAvatar(db.DefaultContext, repo)
	assert.NoError(t, err)

	assert.Equal(t, "", repo.Avatar)
}

func TestGenerateAvatar(t *testing.T) {
	templateRepo := &repo_model.Repository{ID: 10, Avatar: "a"}
	generateRepo := &repo_model.Repository{ID: 11}
	_ = generateAvatar(db.DefaultContext, templateRepo, generateRepo)
	assert.NotEmpty(t, generateRepo.Avatar)
	assert.NotEqual(t, templateRepo.Avatar, generateRepo.Avatar)
}
