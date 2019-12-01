// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIReposGitBlobs(t *testing.T) {
	defer prepareTestEnv(t)()
	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)               // owner of the repo1 & repo16
	user3 := models.AssertExistsAndLoadBean(t, &models.User{ID: 3}).(*models.User)               // owner of the repo3
	user4 := models.AssertExistsAndLoadBean(t, &models.User{ID: 4}).(*models.User)               // owner of neither repos
	repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)   // public repo
	repo3 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)   // public repo
	repo16 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 16}).(*models.Repository) // private repo
	repo1ReadmeSHA := "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	repo3ReadmeSHA := "d56a3073c1dbb7b15963110a049d50cdb5db99fc"
	repo16ReadmeSHA := "f90451c72ef61a7645293d17b47be7a8e983da57"
	badSHA := "0000000000000000000000000000000000000000"

	// Login as User2.
	session := loginUser(t, user2.Name)
	token := getTokenForLoggedInUser(t, session)
	session = emptyTestSession(t) // don't want anyone logged in for this

	// Test a public repo that anyone can GET the blob of
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/%s", user2.Name, repo1.Name, repo1ReadmeSHA)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var gitBlobResponse api.GitBlobResponse
	DecodeJSON(t, resp, &gitBlobResponse)
	assert.NotNil(t, gitBlobResponse)
	expectedContent := "dHJlZSAyYTJmMWQ0NjcwNzI4YTJlMTAwNDllMzQ1YmQ3YTI3NjQ2OGJlYWI2CmF1dGhvciB1c2VyMSA8YWRkcmVzczFAZXhhbXBsZS5jb20+IDE0ODk5NTY0NzkgLTA0MDAKY29tbWl0dGVyIEV0aGFuIEtvZW5pZyA8ZXRoYW50a29lbmlnQGdtYWlsLmNvbT4gMTQ4OTk1NjQ3OSAtMDQwMAoKSW5pdGlhbCBjb21taXQK"
	assert.Equal(t, expectedContent, gitBlobResponse.Content)

	// Tests a private repo with no token so will fail
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/%s", user2.Name, repo16.Name, repo16ReadmeSHA)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Test using access token for a private repo that the user of the token owns
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/%s?token=%s", user2.Name, repo16.Name, repo16ReadmeSHA, token)
	session.MakeRequest(t, req, http.StatusOK)

	// Test using bad sha
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/%s", user2.Name, repo1.Name, badSHA)
	session.MakeRequest(t, req, http.StatusBadRequest)

	// Test using org repo "user3/repo3" where user2 is a collaborator
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/%s?token=%s", user3.Name, repo3.Name, repo3ReadmeSHA, token)
	session.MakeRequest(t, req, http.StatusOK)

	// Test using org repo "user3/repo3" where user2 is a collaborator
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/%s?token=%s", user3.Name, repo3.Name, repo3ReadmeSHA, token)
	session.MakeRequest(t, req, http.StatusOK)

	// Test using org repo "user3/repo3" with no user token
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/%s", user3.Name, repo3ReadmeSHA, repo3.Name)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Login as User4.
	session = loginUser(t, user4.Name)
	token4 := getTokenForLoggedInUser(t, session)
	session = emptyTestSession(t) // don't want anyone logged in for this

	// Test using org repo "user3/repo3" where user4 is a NOT collaborator
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/blobs/d56a3073c1dbb7b15963110a049d50cdb5db99fc?access=%s", user3.Name, repo3.Name, token4)
	session.MakeRequest(t, req, http.StatusNotFound)
}
