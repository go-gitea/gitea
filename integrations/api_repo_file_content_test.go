// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"net/url"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func getExpectedFileContentResponseForFileContents(branch string) *api.FileContentResponse {
	treePath := "README.md"
	sha := "4b4851ad51df6a7d9f25c979345979eaeb5b349f"
	return &api.FileContentResponse{
		Name:        filepath.Base(treePath),
		Path:        treePath,
		SHA:         sha,
		Size:        30,
		URL:         setting.AppURL + "api/v1/repos/user2/repo1/contents/" + treePath,
		HTMLURL:     setting.AppURL + "user2/repo1/blob/" + branch + "/" + treePath,
		GitURL:      setting.AppURL + "api/v1/repos/user2/repo1/git/blobs/" + sha,
		DownloadURL: setting.AppURL + "user2/repo1/raw/branch/" + branch + "/" + treePath,
		Type:        "blob",
		Links: &api.FileLinksResponse{
			Self:    setting.AppURL + "api/v1/repos/user2/repo1/contents/" + treePath,
			GitURL:  setting.AppURL + "api/v1/repos/user2/repo1/git/blobs/" + sha,
			HTMLURL: setting.AppURL + "user2/repo1/blob/" + branch + "/" + treePath,
		},
	}
}

func TestAPIGetFileContents(t *testing.T) {
	onGiteaRun(t, testAPIGetFileContents)
}

func testAPIGetFileContents(t *testing.T, u *url.URL) {
	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)               // owner of the repo1 & repo16
	user3 := models.AssertExistsAndLoadBean(t, &models.User{ID: 3}).(*models.User)               // owner of the repo3, is an org
	user4 := models.AssertExistsAndLoadBean(t, &models.User{ID: 4}).(*models.User)               // owner of neither repos
	repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)   // public repo
	repo3 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)   // public repo
	repo16 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 16}).(*models.Repository) // private repo
	treePath := "README.md"

	// Get user2's token
	session := loginUser(t, user2.Name)
	token2 := getTokenForLoggedInUser(t, session)
	session = emptyTestSession(t)
	// Get user4's token
	session = loginUser(t, user4.Name)
	token4 := getTokenForLoggedInUser(t, session)
	session = emptyTestSession(t)

	// Make a second master branch in repo1
	repo1.CreateNewBranch(user2, repo1.DefaultBranch, "master2")

	// ref is default branch
	branch := repo1.DefaultBranch
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, branch)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var fileContentResponse api.FileContentResponse
	DecodeJSON(t, resp, &fileContentResponse)
	assert.NotNil(t, fileContentResponse)
	expectedFileContentResponse := getExpectedFileContentResponseForFileContents(branch)
	assert.EqualValues(t, *expectedFileContentResponse, fileContentResponse)

	// No ref
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &fileContentResponse)
	assert.NotNil(t, fileContentResponse)
	expectedFileContentResponse = getExpectedFileContentResponseForFileContents(repo1.DefaultBranch)
	assert.EqualValues(t, *expectedFileContentResponse, fileContentResponse)

	// ref is master2
	branch = "master2"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, branch)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &fileContentResponse)
	assert.NotNil(t, fileContentResponse)
	expectedFileContentResponse = getExpectedFileContentResponseForFileContents("master2")
	assert.EqualValues(t, *expectedFileContentResponse, fileContentResponse)

	// Test file contents a file with the wrong branch
	branch = "badbranch"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, branch)
	resp = session.MakeRequest(t, req, http.StatusInternalServerError)
	expectedAPIError := context.APIError{
		Message: "object does not exist [id: " + branch + ", rel_path: ]",
		URL:     base.DocURL,
	}
	var apiError context.APIError
	DecodeJSON(t, resp, &apiError)
	assert.Equal(t, expectedAPIError, apiError)

	// Test accessing private branch with user token that does not have access - should fail
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token4)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Test access private branch of owner of token
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/readme.md?token=%s", user2.Name, repo16.Name, token2)
	session.MakeRequest(t, req, http.StatusOK)

	// Test access of org user3 private repo file by owner user2
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?token=%s", user3.Name, repo3.Name, treePath, token2)
	session.MakeRequest(t, req, http.StatusOK)
}
