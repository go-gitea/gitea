// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	auth_model "codenest.dev/models/auth"
	repo_model "codenest.dev/models/repo"
	"codenest.dev/models/unittest"
	user_model "codenest.dev/models/user"
	"codenest.dev/modules/setting"
	api "codenest.dev/modules/structs"
	"codenest.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIUpdateRepoAvatar(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	token := getUserToken(t, user2.LowerName, auth_model.AccessTokenScopeWriteRepository)

	// Test what happens if you use a valid image
	avatar, err := os.ReadFile(filepath.Join(setting.GetCodeNestTestSourceRoot(), "tests/integration/avatar.png"))
	assert.NoError(t, err)
	if err != nil {
		assert.FailNow(t, "Unable to open avatar.png")
	}

	opts := api.UpdateRepoAvatarOption{
		Image: base64.StdEncoding.EncodeToString(avatar),
	}

	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/avatar", repo.OwnerName, repo.Name), &opts).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Test what happens if you don't have a valid Base64 string
	opts = api.UpdateRepoAvatarOption{
		Image: "Invalid",
	}

	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/avatar", repo.OwnerName, repo.Name), &opts).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusBadRequest)

	// Test what happens if you use a file that is not an image
	opts = api.UpdateRepoAvatarOption{
		Image: base64.StdEncoding.EncodeToString([]byte("This is not an image")),
	}

	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/avatar", repo.OwnerName, repo.Name), &opts).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusInternalServerError)
}

func TestAPIDeleteRepoAvatar(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	token := getUserToken(t, user2.LowerName, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/%s/avatar", repo.OwnerName, repo.Name)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
}
