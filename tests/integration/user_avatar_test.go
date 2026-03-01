// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserAvatar(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	img := avatar.RandomImageDefaultSize([]byte("any-random-image-seed"))
	originAvatarData := &bytes.Buffer{}
	require.NoError(t, png.Encode(originAvatarData, img))

	// setup multipart form to upload avatar
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("source", "local")
	part, err := writer.CreateFormFile("avatar", "avatar-for-testuseravatar.png")
	require.NoError(t, err)
	_, _ = io.Copy(part, bytes.NewReader(originAvatarData.Bytes()))
	require.NoError(t, writer.Close())

	// upload avatar
	req := NewRequestWithBody(t, "POST", "/user/settings/avatar", body)
	req.Header.Add("Content-Type", writer.FormDataContentType())
	session.MakeRequest(t, req, http.StatusSeeOther)

	// check user2's avatar can be accessed
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	req = NewRequest(t, "GET", user2.AvatarLinkWithSize(t.Context(), 0))
	_ = session.MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", "/user2.png")
	resp := MakeRequest(t, req, http.StatusSeeOther)
	avatarRedirect := resp.Header().Get("Location")
	assert.Equal(t, "/avatars/"+user2.Avatar, avatarRedirect)

	// check the content of the avatar is correct
	resp = MakeRequest(t, NewRequest(t, "GET", avatarRedirect), http.StatusOK)
	assert.Equal(t, "image/png", resp.Header().Get("Content-Type"))
	avatarData, _ := io.ReadAll(resp.Body)
	assert.Equal(t, originAvatarData.Bytes(), avatarData)

	// for non-existing avatar, it should return a random one with proper cache control headers
	resp = MakeRequest(t, NewRequest(t, "GET", "/avatars/no-such-avatar"), http.StatusOK)
	assert.Equal(t, "image/png", resp.Header().Get("Content-Type"))
	assert.NotEmpty(t, resp.Header().Get("ETag"))
	assert.NotEmpty(t, resp.Header().Get("Last-Modified"))
	assert.Contains(t, resp.Header().Get("Cache-Control"), "public")
	avatarData, _ = io.ReadAll(resp.Body)
	_, err = png.Decode(bytes.NewReader(avatarData))
	require.NoError(t, err)
}
