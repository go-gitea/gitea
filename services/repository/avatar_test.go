// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/avatar"

	"github.com/stretchr/testify/assert"
)

func TestUploadAvatar(t *testing.T) {
	for _, testCase := range []struct {
		width        int
		deleteAvatar bool
	}{
		{1, false},
		{5000, false},
		{1, true},
	} {
		// Generate image
		var buff bytes.Buffer
		png.Encode(&buff, image.NewRGBA(image.Rect(0, 0, testCase.width, 1)))

		assert.NoError(t, unittest.PrepareTestDatabase())
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})

		err := UploadAvatar(t.Context(), repo, buff.Bytes())
		if testCase.width == 5000 {
			assert.Error(t, err)
			continue
		}
		assert.NoError(t, err)
		if testCase.deleteAvatar {
			err = DeleteAvatar(t.Context(), repo)
			assert.NoError(t, err)
			assert.Empty(t, repo.Avatar)
		} else {
			assert.Equal(t, avatar.HashAvatar(10, buff.Bytes()), repo.Avatar)
		}
	}
}

func TestGenerateAvatar(t *testing.T) {
	templateRepo := &repo_model.Repository{ID: 10, Avatar: "a"}
	generateRepo := &repo_model.Repository{ID: 11}
	_ = generateAvatar(t.Context(), templateRepo, generateRepo)
	assert.NotEmpty(t, generateRepo.Avatar)
	assert.NotEqual(t, templateRepo.Avatar, generateRepo.Avatar)
}
