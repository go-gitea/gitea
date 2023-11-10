// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIReposGitBlobs(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})         // owner of the repo1 & repo16
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})          // owner of the repo3
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})         // owner of neither repos
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})   // public repo
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})   // public repo
	repo16 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16}) // private repo
	repo1ReadmeSHA := "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	repo3ReadmeSHA := "d56a3073c1dbb7b15963110a049d50cdb5db99fc"
	repo16ReadmeSHA := "f90451c72ef61a7645293d17b47be7a8e983da57"
	badSHA := "0000000000000000000000000000000000000000"

	// Login as User2.
	session := loginUser(t, user2.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	// Test a public repo that anyone can GET the blob of
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/%s", user2.Name, repo1.Name, repo1ReadmeSHA)
	resp := MakeRequest(t, req, http.StatusOK)
	var gitBlobResponse api.GitBlobResponse
	DecodeJSON(t, resp, &gitBlobResponse)
	assert.NotNil(t, gitBlobResponse)
	expectedContent := "dHJlZSAyYTJmMWQ0NjcwNzI4YTJlMTAwNDllMzQ1YmQ3YTI3NjQ2OGJlYWI2CmF1dGhvciB1c2VyMSA8YWRkcmVzczFAZXhhbXBsZS5jb20+IDE0ODk5NTY0NzkgLTA0MDAKY29tbWl0dGVyIEV0aGFuIEtvZW5pZyA8ZXRoYW50a29lbmlnQGdtYWlsLmNvbT4gMTQ4OTk1NjQ3OSAtMDQwMAoKSW5pdGlhbCBjb21taXQK"
	assert.Equal(t, expectedContent, gitBlobResponse.Content)

	// Tests a private repo with no token so will fail
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/%s", user2.Name, repo16.Name, repo16ReadmeSHA)
	MakeRequest(t, req, http.StatusNotFound)

	// Test using access token for a private repo that the user of the token owns
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/%s?token=%s", user2.Name, repo16.Name, repo16ReadmeSHA, token)
	MakeRequest(t, req, http.StatusOK)

	// Test using bad sha
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/%s", user2.Name, repo1.Name, badSHA)
	MakeRequest(t, req, http.StatusBadRequest)

	// Test using org repo "org3/repo3" where user2 is a collaborator
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/%s?token=%s", org3.Name, repo3.Name, repo3ReadmeSHA, token)
	MakeRequest(t, req, http.StatusOK)

	// Test using org repo "org3/repo3" where user2 is a collaborator
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/%s?token=%s", org3.Name, repo3.Name, repo3ReadmeSHA, token)
	MakeRequest(t, req, http.StatusOK)

	// Test using org repo "org3/repo3" with no user token
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/%s", org3.Name, repo3ReadmeSHA, repo3.Name)
	MakeRequest(t, req, http.StatusNotFound)

	// Login as User4.
	session = loginUser(t, user4.Name)
	token4 := getTokenForLoggedInUser(t, session)

	// Test using org repo "org3/repo3" where user4 is a NOT collaborator
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/d56a3073c1dbb7b15963110a049d50cdb5db99fc?access=%s", org3.Name, repo3.Name, token4)
	MakeRequest(t, req, http.StatusNotFound)
}
