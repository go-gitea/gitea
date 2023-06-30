// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIUpdateRepoAvatar(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	token := getUserToken(t, user2.LowerName, auth_model.AccessTokenScopeWriteRepository)

	// Test what happens if you use a valid image
	avatar, err := os.ReadFile("tests/integration/avatar.png")
	assert.NoError(t, err)
	if err != nil {
		assert.FailNow(t, "Unable to open avatar.png")
	}

	opts := api.UpdateRepoAvatarOption{
		Image: base64.StdEncoding.EncodeToString(avatar),
	}

	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/avatar?token=%s", repo.OwnerName, repo.Name, token), &opts)
	MakeRequest(t, req, http.StatusNoContent)

	// Test what happens if you don't have a valid Base64 string
	opts = api.UpdateRepoAvatarOption{
		Image: "Invalid",
	}

	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/avatar?token=%s", repo.OwnerName, repo.Name, token), &opts)
	MakeRequest(t, req, http.StatusBadRequest)

	// Test what happens if you use a file that is not an image
	text, err := os.ReadFile("tests/integration/README.md")
	assert.NoError(t, err)
	if err != nil {
		assert.FailNow(t, "Unable to open README.md")
	}

	opts = api.UpdateRepoAvatarOption{
		Image: base64.StdEncoding.EncodeToString(text),
	}

	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/avatar?token=%s", repo.OwnerName, repo.Name, token), &opts)
	MakeRequest(t, req, http.StatusInternalServerError)
}

func TestAPIDeleteRepoAvatar(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	token := getUserToken(t, user2.LowerName, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/%s/avatar?token=%s", repo.OwnerName, repo.Name, token))
	MakeRequest(t, req, http.StatusNoContent)
}
