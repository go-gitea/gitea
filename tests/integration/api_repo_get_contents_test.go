// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"io"
	"net/http"
	"net/url"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/stretchr/testify/assert"
)

func getExpectedContentsResponseForContents(ref, refType, lastCommitSHA string) *api.ContentsResponse {
	treePath := "README.md"
	sha := "4b4851ad51df6a7d9f25c979345979eaeb5b349f"
	encoding := "base64"
	content := "IyByZXBvMQoKRGVzY3JpcHRpb24gZm9yIHJlcG8x"
	selfURL := setting.AppURL + "api/v1/repos/user2/repo1/contents/" + treePath + "?ref=" + ref
	htmlURL := setting.AppURL + "user2/repo1/src/" + refType + "/" + ref + "/" + treePath
	gitURL := setting.AppURL + "api/v1/repos/user2/repo1/git/blobs/" + sha
	downloadURL := setting.AppURL + "user2/repo1/raw/" + refType + "/" + ref + "/" + treePath
	return &api.ContentsResponse{
		Name:          treePath,
		Path:          treePath,
		SHA:           sha,
		LastCommitSHA: lastCommitSHA,
		Type:          "file",
		Size:          30,
		Encoding:      &encoding,
		Content:       &content,
		URL:           &selfURL,
		HTMLURL:       &htmlURL,
		GitURL:        &gitURL,
		DownloadURL:   &downloadURL,
		Links: &api.FileLinksResponse{
			Self:    &selfURL,
			GitURL:  &gitURL,
			HTMLURL: &htmlURL,
		},
	}
}

func TestAPIGetContents(t *testing.T) {
	onGiteaRun(t, testAPIGetContents)
}

func testAPIGetContents(t *testing.T, u *url.URL) {
	/*** SETUP ***/
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})         // owner of the repo1 & repo16
	user3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})         // owner of the repo3, is an org
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})         // owner of neither repos
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})   // public repo
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})   // public repo
	repo16 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16}) // private repo
	treePath := "README.md"

	// Get user2's token
	session := loginUser(t, user2.Name)
	token2 := getTokenForLoggedInUser(t, session)
	// Get user4's token
	session = loginUser(t, user4.Name)
	token4 := getTokenForLoggedInUser(t, session)

	// Make a new branch in repo1
	newBranch := "test_branch"
	err := repo_service.CreateNewBranch(git.DefaultContext, user2, repo1, repo1.DefaultBranch, newBranch)
	assert.NoError(t, err)
	// Get the commit ID of the default branch
	gitRepo, err := git.OpenRepository(git.DefaultContext, repo1.RepoPath())
	assert.NoError(t, err)
	defer gitRepo.Close()

	commitID, err := gitRepo.GetBranchCommitID(repo1.DefaultBranch)
	assert.NoError(t, err)
	// Make a new tag in repo1
	newTag := "test_tag"
	err = gitRepo.CreateTag(newTag, commitID)
	assert.NoError(t, err)
	/*** END SETUP ***/

	// ref is default ref
	ref := repo1.DefaultBranch
	refType := "branch"
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref)
	resp := MakeRequest(t, req, http.StatusOK)
	var contentsResponse api.ContentsResponse
	DecodeJSON(t, resp, &contentsResponse)
	assert.NotNil(t, contentsResponse)
	lastCommit, _ := gitRepo.GetCommitByPath("README.md")
	expectedContentsResponse := getExpectedContentsResponseForContents(ref, refType, lastCommit.ID.String())
	assert.EqualValues(t, *expectedContentsResponse, contentsResponse)

	// No ref
	refType = "branch"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsResponse)
	assert.NotNil(t, contentsResponse)
	expectedContentsResponse = getExpectedContentsResponseForContents(repo1.DefaultBranch, refType, lastCommit.ID.String())
	assert.EqualValues(t, *expectedContentsResponse, contentsResponse)

	// ref is the branch we created above  in setup
	ref = newBranch
	refType = "branch"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsResponse)
	assert.NotNil(t, contentsResponse)
	branchCommit, _ := gitRepo.GetBranchCommit(ref)
	lastCommit, _ = branchCommit.GetCommitByPath("README.md")
	expectedContentsResponse = getExpectedContentsResponseForContents(ref, refType, lastCommit.ID.String())
	assert.EqualValues(t, *expectedContentsResponse, contentsResponse)

	// ref is the new tag we created above in setup
	ref = newTag
	refType = "tag"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsResponse)
	assert.NotNil(t, contentsResponse)
	tagCommit, _ := gitRepo.GetTagCommit(ref)
	lastCommit, _ = tagCommit.GetCommitByPath("README.md")
	expectedContentsResponse = getExpectedContentsResponseForContents(ref, refType, lastCommit.ID.String())
	assert.EqualValues(t, *expectedContentsResponse, contentsResponse)

	// ref is a commit
	ref = commitID
	refType = "commit"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsResponse)
	assert.NotNil(t, contentsResponse)
	expectedContentsResponse = getExpectedContentsResponseForContents(ref, refType, commitID)
	assert.EqualValues(t, *expectedContentsResponse, contentsResponse)

	// Test file contents a file with a bad ref
	ref = "badref"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref)
	MakeRequest(t, req, http.StatusNotFound)

	// Test accessing private ref with user token that does not have access - should fail
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token4)
	MakeRequest(t, req, http.StatusNotFound)

	// Test access private ref of owner of token
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/readme.md?token=%s", user2.Name, repo16.Name, token2)
	MakeRequest(t, req, http.StatusOK)

	// Test access of org user3 private repo file by owner user2
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?token=%s", user3.Name, repo3.Name, treePath, token2)
	MakeRequest(t, req, http.StatusOK)
}

func TestAPIGetContentsRefFormats(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		file := "README.md"
		sha := "65f1bf27bc3bf70f64657658635e66094edbcb4d"
		content := "# repo1\n\nDescription for repo1"

		noRef := setting.AppURL + "api/v1/repos/user2/repo1/raw/" + file
		refInPath := setting.AppURL + "api/v1/repos/user2/repo1/raw/" + sha + "/" + file
		refInQuery := setting.AppURL + "api/v1/repos/user2/repo1/raw/" + file + "?ref=" + sha

		resp := MakeRequest(t, NewRequest(t, http.MethodGet, noRef), http.StatusOK)
		raw, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.EqualValues(t, content, string(raw))

		resp = MakeRequest(t, NewRequest(t, http.MethodGet, refInPath), http.StatusOK)
		raw, err = io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.EqualValues(t, content, string(raw))

		resp = MakeRequest(t, NewRequest(t, http.MethodGet, refInQuery), http.StatusOK)
		raw, err = io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.EqualValues(t, content, string(raw))
	})
}
