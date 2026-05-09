// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestViewHTTPSDeployKeysNoLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/https_keys")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestCreateHTTPSDeployKeyNoLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/https_keys", api.CreateHTTPSDeployKeyOption{
		Name:     "test-key",
		ReadOnly: true,
	})
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestGetHTTPSDeployKeyNoLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/https_keys/1")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestDeleteHTTPSDeployKeyNoLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequest(t, "DELETE", "/api/v1/repos/user2/repo1/https_keys/1")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestCreateHTTPSDeployKey(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: "repo1"})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	keysURL := fmt.Sprintf("/api/v1/repos/%s/%s/https_keys", repoOwner.Name, repo.Name)

	createBody := api.CreateHTTPSDeployKeyOption{
		Name:     "ci-read-only",
		ReadOnly: true,
	}
	req := NewRequestWithJSON(t, "POST", keysURL, createBody).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)

	createdKey := DecodeJSON(t, resp, &api.HTTPSDeployKey{})
	assert.True(t, createdKey.ReadOnly)
	assert.Equal(t, "ci-read-only", createdKey.Name)
	assert.NotEmpty(t, createdKey.Token, "create response must include the plaintext token")
	assert.NotEmpty(t, createdKey.TokenLastEight)
	assert.NotEmpty(t, createdKey.URL)
	assert.Equal(t, createdKey.Token[len(createdKey.Token)-8:], createdKey.TokenLastEight)

	unittest.AssertExistsAndLoadBean(t, &asymkey_model.HTTPSDeployKey{
		ID:   createdKey.ID,
		Name: "ci-read-only",
	})
}

func TestListHTTPSDeployKeys(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: "repo1"})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	keysURL := fmt.Sprintf("/api/v1/repos/%s/%s/https_keys", repoOwner.Name, repo.Name)

	// Create a key
	req := NewRequestWithJSON(t, "POST", keysURL, api.CreateHTTPSDeployKeyOption{
		Name:     "list-test-key",
		ReadOnly: false,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)

	// List should now contain the key
	req = NewRequest(t, "GET", keysURL).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	keys := DecodeJSON(t, resp, []api.HTTPSDeployKey{})
	found := false
	for _, k := range keys {
		if k.Name == "list-test-key" {
			found = true
			assert.False(t, k.ReadOnly)
			assert.Empty(t, k.Token, "list response must not include the plaintext token")
			break
		}
	}
	assert.True(t, found, "created key should appear in list")
}

func TestGetHTTPSDeployKey(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: "repo1"})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	keysURL := fmt.Sprintf("/api/v1/repos/%s/%s/https_keys", repoOwner.Name, repo.Name)

	// Create a key
	createBody := api.CreateHTTPSDeployKeyOption{
		Name:     "get-test-key",
		ReadOnly: true,
	}
	req := NewRequestWithJSON(t, "POST", keysURL, createBody).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	createdKey := DecodeJSON(t, resp, &api.HTTPSDeployKey{})

	// Get by ID
	getURL := fmt.Sprintf("%s/%d", keysURL, createdKey.ID)
	req = NewRequest(t, "GET", getURL).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	gotKey := DecodeJSON(t, resp, &api.HTTPSDeployKey{})
	assert.Equal(t, createdKey.ID, gotKey.ID)
	assert.Equal(t, "get-test-key", gotKey.Name)
	assert.True(t, gotKey.ReadOnly)
	assert.Empty(t, gotKey.Token, "get response must not include the plaintext token")

	// Get non-existent key
	req = NewRequest(t, "GET", keysURL+"/999999").
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestDeleteHTTPSDeployKey(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: "repo1"})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	keysURL := fmt.Sprintf("/api/v1/repos/%s/%s/https_keys", repoOwner.Name, repo.Name)

	// Create a key
	createBody := api.CreateHTTPSDeployKeyOption{
		Name:     "delete-test-key",
		ReadOnly: true,
	}
	req := NewRequestWithJSON(t, "POST", keysURL, createBody).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	createdKey := DecodeJSON(t, resp, &api.HTTPSDeployKey{})

	// Delete
	deleteURL := fmt.Sprintf("%s/%d", keysURL, createdKey.ID)
	req = NewRequest(t, "DELETE", deleteURL).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Verify deleted
	req = NewRequest(t, "GET", deleteURL).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestCreateHTTPSDeployKeyDuplicateName(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: "repo1"})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	keysURL := fmt.Sprintf("/api/v1/repos/%s/%s/https_keys", repoOwner.Name, repo.Name)

	createBody := api.CreateHTTPSDeployKeyOption{
		Name:     "duplicate-name-key",
		ReadOnly: true,
	}
	req := NewRequestWithJSON(t, "POST", keysURL, createBody).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)

	// Try to create with the same name
	req = NewRequestWithJSON(t, "POST", keysURL, createBody).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)
}
