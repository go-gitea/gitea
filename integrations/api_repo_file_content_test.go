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
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func getExpectedFileContentResponseForFileContents(ref, refType string) *api.FileContentResponse {
	treePath := "README.md"
	sha := "4b4851ad51df6a7d9f25c979345979eaeb5b349f"
	return &api.FileContentResponse{
		Name:        filepath.Base(treePath),
		Path:        treePath,
		SHA:         sha,
		Size:        30,
		URL:         setting.AppURL + "api/v1/repos/user2/repo1/contents/" + treePath + "?ref=" + ref,
		HTMLURL:     setting.AppURL + "user2/repo1/src/" + refType + "/" + ref + "/" + treePath,
		GitURL:      setting.AppURL + "api/v1/repos/user2/repo1/git/blobs/" + sha,
		DownloadURL: setting.AppURL + "user2/repo1/raw/" + refType + "/" + ref + "/" + treePath,
		Type:        "blob",
		Encoding:    "base64",
		Content:     "IyByZXBvMQoKRGVzY3JpcHRpb24gZm9yIHJlcG8x",
		Links: &api.FileLinksResponse{
			Self:    setting.AppURL + "api/v1/repos/user2/repo1/contents/" + treePath + "?ref=" + ref,
			GitURL:  setting.AppURL + "api/v1/repos/user2/repo1/git/blobs/" + sha,
			HTMLURL: setting.AppURL + "user2/repo1/src/" + refType + "/" + ref + "/" + treePath,
		},
	}
}

func TestAPIGetFileContents(t *testing.T) {
	onGiteaRun(t, testAPIGetFileContents)
}

func testAPIGetFileContents(t *testing.T, u *url.URL) {
	/*** SETUP ***/
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

	// Make a new branch in repo1
	newBranch := "test_branch"
	repo1.CreateNewBranch(user2, repo1.DefaultBranch, newBranch)
	// Get the commit ID of the default branch
	gitRepo, _ := git.OpenRepository(repo1.RepoPath())
	commitID, _ := gitRepo.GetBranchCommitID(repo1.DefaultBranch)
	// Make a new tag in repo1
	newTag := "test_tag"
	gitRepo.CreateTag(newTag, commitID)
	/*** END SETUP ***/

	// ref is default ref
	ref := repo1.DefaultBranch
	refType := "branch"
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var fileContentResponse api.FileContentResponse
	DecodeJSON(t, resp, &fileContentResponse)
	assert.NotNil(t, fileContentResponse)
	expectedFileContentResponse := getExpectedFileContentResponseForFileContents(ref, refType)
	assert.EqualValues(t, *expectedFileContentResponse, fileContentResponse)

	// No ref
	refType = "branch"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &fileContentResponse)
	assert.NotNil(t, fileContentResponse)
	expectedFileContentResponse = getExpectedFileContentResponseForFileContents(repo1.DefaultBranch, refType)
	assert.EqualValues(t, *expectedFileContentResponse, fileContentResponse)

	// ref is the branch we created above  in setup
	ref = newBranch
	refType = "branch"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &fileContentResponse)
	assert.NotNil(t, fileContentResponse)
	expectedFileContentResponse = getExpectedFileContentResponseForFileContents(ref, refType)
	assert.EqualValues(t, *expectedFileContentResponse, fileContentResponse)

	// ref is the new tag we created above in setup
	ref = newTag
	refType = "tag"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &fileContentResponse)
	assert.NotNil(t, fileContentResponse)
	expectedFileContentResponse = getExpectedFileContentResponseForFileContents(ref, refType)
	assert.EqualValues(t, *expectedFileContentResponse, fileContentResponse)

	// ref is a commit
	ref = commitID
	refType = "commit"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &fileContentResponse)
	assert.NotNil(t, fileContentResponse)
	expectedFileContentResponse = getExpectedFileContentResponseForFileContents(ref, refType)
	assert.EqualValues(t, *expectedFileContentResponse, fileContentResponse)

	// Test file contents a file with a bad ref
	ref = "badref"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref)
	resp = session.MakeRequest(t, req, http.StatusInternalServerError)
	expectedAPIError := context.APIError{
		Message: "object does not exist [id: " + ref + ", rel_path: ]",
		URL:     setting.API.SwaggerURL,
	}
	var apiError context.APIError
	DecodeJSON(t, resp, &apiError)
	assert.Equal(t, expectedAPIError, apiError)

	// Test accessing private ref with user token that does not have access - should fail
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token4)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Test access private ref of owner of token
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/readme.md?token=%s", user2.Name, repo16.Name, token2)
	session.MakeRequest(t, req, http.StatusOK)

	// Test access of org user3 private repo file by owner user2
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?token=%s", user3.Name, repo3.Name, treePath, token2)
	session.MakeRequest(t, req, http.StatusOK)
}
