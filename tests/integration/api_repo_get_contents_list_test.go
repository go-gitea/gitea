// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"path"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/stretchr/testify/assert"
)

func getExpectedContentsListResponseForContents(ref, refType, lastCommitSHA string) []*api.ContentsResponse {
	treePath := "README.md"
	sha := "4b4851ad51df6a7d9f25c979345979eaeb5b349f"
	selfURL := setting.AppURL + "api/v1/repos/user2/repo1/contents/" + treePath + "?ref=" + ref
	htmlURL := setting.AppURL + "user2/repo1/src/" + refType + "/" + ref + "/" + treePath
	gitURL := setting.AppURL + "api/v1/repos/user2/repo1/git/blobs/" + sha
	downloadURL := setting.AppURL + "user2/repo1/raw/" + refType + "/" + ref + "/" + treePath
	return []*api.ContentsResponse{
		{
			Name:              path.Base(treePath),
			Path:              treePath,
			SHA:               sha,
			LastCommitSHA:     util.ToPointer(lastCommitSHA),
			LastCommitterDate: util.ToPointer(time.Date(2017, time.March, 19, 16, 47, 59, 0, time.FixedZone("", -14400))),
			LastAuthorDate:    util.ToPointer(time.Date(2017, time.March, 19, 16, 47, 59, 0, time.FixedZone("", -14400))),
			Type:              "file",
			Size:              30,
			URL:               &selfURL,
			HTMLURL:           &htmlURL,
			GitURL:            &gitURL,
			DownloadURL:       &downloadURL,
			Links: &api.FileLinksResponse{
				Self:    &selfURL,
				GitURL:  &gitURL,
				HTMLURL: &htmlURL,
			},
		},
	}
}

func TestAPIGetContentsList(t *testing.T) {
	onGiteaRun(t, testAPIGetContentsList)
}

func testAPIGetContentsList(t *testing.T, u *url.URL) {
	/*** SETUP ***/
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})         // owner of the repo1 & repo16
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})          // owner of the repo3, is an org
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})         // owner of neither repos
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})   // public repo
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})   // public repo
	repo16 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16}) // private repo

	// Get user2's token
	session := loginUser(t, user2.Name)
	token2 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)
	// Get user4's token
	session = loginUser(t, user4.Name)
	token4 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	// Get the commit ID of the default branch
	gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo1)
	assert.NoError(t, err)
	defer gitRepo.Close()

	// Make a new branch in repo1
	newBranch := "test_branch"
	err = repo_service.CreateNewBranch(git.DefaultContext, user2, repo1, gitRepo, repo1.DefaultBranch, newBranch)
	assert.NoError(t, err)

	commitID, _ := gitRepo.GetBranchCommitID(repo1.DefaultBranch)
	// Make a new tag in repo1
	newTag := "test_tag"
	err = gitRepo.CreateTag(newTag, commitID)
	assert.NoError(t, err)
	/*** END SETUP ***/

	// ref is default ref
	ref := repo1.DefaultBranch
	refType := "branch"
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents?ref=%s", user2.Name, repo1.Name, ref)
	resp := MakeRequest(t, req, http.StatusOK)
	var contentsListResponse []*api.ContentsResponse
	DecodeJSON(t, resp, &contentsListResponse)
	assert.NotNil(t, contentsListResponse)
	lastCommit, err := gitRepo.GetCommitByPath("README.md")
	assert.NoError(t, err)
	expectedContentsListResponse := getExpectedContentsListResponseForContents(ref, refType, lastCommit.ID.String())
	assert.Equal(t, expectedContentsListResponse, contentsListResponse)

	// No ref
	refType = "branch"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/", user2.Name, repo1.Name)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsListResponse)
	assert.NotNil(t, contentsListResponse)

	expectedContentsListResponse = getExpectedContentsListResponseForContents(repo1.DefaultBranch, refType, lastCommit.ID.String())
	assert.Equal(t, expectedContentsListResponse, contentsListResponse)

	// ref is the branch we created above in setup
	ref = newBranch
	refType = "branch"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents?ref=%s", user2.Name, repo1.Name, ref)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsListResponse)
	assert.NotNil(t, contentsListResponse)
	branchCommit, err := gitRepo.GetBranchCommit(ref)
	assert.NoError(t, err)
	lastCommit, err = branchCommit.GetCommitByPath("README.md")
	assert.NoError(t, err)
	expectedContentsListResponse = getExpectedContentsListResponseForContents(ref, refType, lastCommit.ID.String())
	assert.Equal(t, expectedContentsListResponse, contentsListResponse)

	// ref is the new tag we created above in setup
	ref = newTag
	refType = "tag"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/?ref=%s", user2.Name, repo1.Name, ref)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsListResponse)
	assert.NotNil(t, contentsListResponse)
	tagCommit, err := gitRepo.GetTagCommit(ref)
	assert.NoError(t, err)
	lastCommit, err = tagCommit.GetCommitByPath("README.md")
	assert.NoError(t, err)
	expectedContentsListResponse = getExpectedContentsListResponseForContents(ref, refType, lastCommit.ID.String())
	assert.Equal(t, expectedContentsListResponse, contentsListResponse)

	// ref is a commit
	ref = commitID
	refType = "commit"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/?ref=%s", user2.Name, repo1.Name, ref)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsListResponse)
	assert.NotNil(t, contentsListResponse)
	expectedContentsListResponse = getExpectedContentsListResponseForContents(ref, refType, commitID)
	assert.Equal(t, expectedContentsListResponse, contentsListResponse)

	// Test file contents a file with a bad ref
	ref = "badref"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/?ref=%s", user2.Name, repo1.Name, ref)
	MakeRequest(t, req, http.StatusNotFound)

	// Test accessing private ref with user token that does not have access - should fail
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/", user2.Name, repo16.Name).
		AddTokenAuth(token4)
	MakeRequest(t, req, http.StatusNotFound)

	// Test access private ref of owner of token
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/", user2.Name, repo16.Name).
		AddTokenAuth(token2)
	MakeRequest(t, req, http.StatusOK)

	// Test access of org org3 private repo file by owner user2
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/", org3.Name, repo3.Name).
		AddTokenAuth(token2)
	MakeRequest(t, req, http.StatusOK)
}
