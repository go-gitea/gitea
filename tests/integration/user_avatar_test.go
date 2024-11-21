// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/avatar"

	"github.com/stretchr/testify/assert"
)

func TestUserAvatar(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}) // owner of the repo3, is an org

		seed := user2.Email
		if len(seed) == 0 {
			seed = user2.Name
		}

		img, err := avatar.RandomImage([]byte(seed))
		if err != nil {
			assert.NoError(t, err)
			return
		}

		session := loginUser(t, "user2")
		csrf := GetUserCSRFToken(t, session)

		imgData := &bytes.Buffer{}

		body := &bytes.Buffer{}

		// Setup multi-part
		writer := multipart.NewWriter(body)
		writer.WriteField("source", "local")
		part, err := writer.CreateFormFile("avatar", "avatar-for-testuseravatar.png")
		if err != nil {
			assert.NoError(t, err)
			return
		}

		if err := png.Encode(imgData, img); err != nil {
			assert.NoError(t, err)
			return
		}

		if _, err := io.Copy(part, imgData); err != nil {
			assert.NoError(t, err)
			return
		}

		if err := writer.Close(); err != nil {
			assert.NoError(t, err)
			return
		}

		req := NewRequestWithBody(t, "POST", "/user/settings/avatar", body)
		req.Header.Add("X-Csrf-Token", csrf)
		req.Header.Add("Content-Type", writer.FormDataContentType())

		session.MakeRequest(t, req, http.StatusSeeOther)

		user2 = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}) // owner of the repo3, is an org

		req = NewRequest(t, "GET", user2.AvatarLinkWithSize(db.DefaultContext, 0))
		_ = session.MakeRequest(t, req, http.StatusOK)

		testGetAvatarRedirect(t, user2)

		// Can't test if the response matches because the image is re-generated on upload but checking that this at least doesn't give a 404 should be enough.
	})
}

func testGetAvatarRedirect(t *testing.T, user *user_model.User) {
	t.Run(fmt.Sprintf("getAvatarRedirect_%s", user.Name), func(t *testing.T) {
		req := NewRequestf(t, "GET", "/%s.png", user.Name)
		resp := MakeRequest(t, req, http.StatusSeeOther)
		assert.EqualValues(t, fmt.Sprintf("/avatars/%s", user.Avatar), resp.Header().Get("location"))
	})
}
