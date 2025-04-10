// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	api "code.gitea.io/gitea/modules/structs"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/stretchr/testify/assert"
)

func getExpectedcontentsListResponseForFiles(ref, refType, lastCommitSHA string) []*api.ContentsResponse {
	return []*api.ContentsResponse{getExpectedContentsResponseForContents(ref, refType, lastCommitSHA)}
}

func TestAPIGetRequestedFiles(t *testing.T) {
	onGiteaRun(t, testAPIGetRequestedFiles)
}

func testAPIGetRequestedFiles(t *testing.T, u *url.URL) {
	/*** SETUP ***/
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})         // owner of the repo1 & repo16
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})          // owner of the repo3, is an org
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})         // owner of neither repos
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})   // public repo
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})   // public repo
	repo16 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16}) // private repo
	filesOptions := &api.GetFilesOptions{
		Files: []string{
			"README.md",
		},
	}

	// Get user2's token	req.Body =
	session := loginUser(t, user2.Name)
	token2 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository) //TODO: allow for a POST-request to be scope read
	// Get user4's token
	session = loginUser(t, user4.Name)
	token4 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository) //TODO: allow for a POST-request to be scope read

	// Get the commit ID of the default branch
	gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo1)
	assert.NoError(t, err)
	defer gitRepo.Close()

	// Make a new branch in repo1
	newBranch := "test_branch"
	err = repo_service.CreateNewBranch(git.DefaultContext, user2, repo1, gitRepo, repo1.DefaultBranch, newBranch)
	assert.NoError(t, err)

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
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/files?ref=%s", user2.Name, repo1.Name, ref), &filesOptions)
	resp := MakeRequest(t, req, http.StatusOK)
	var contentsListResponse []*api.ContentsResponse
	DecodeJSON(t, resp, &contentsListResponse)
	assert.NotNil(t, contentsListResponse)
	lastCommit, _ := gitRepo.GetCommitByPath("README.md")
	expectedcontentsListResponse := getExpectedcontentsListResponseForFiles(ref, refType, lastCommit.ID.String())
	assert.Equal(t, expectedcontentsListResponse, contentsListResponse)

	// No ref
	refType = "branch"
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/files", user2.Name, repo1.Name), &filesOptions)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsListResponse)
	assert.NotNil(t, contentsListResponse)
	expectedcontentsListResponse = getExpectedcontentsListResponseForFiles(repo1.DefaultBranch, refType, lastCommit.ID.String())
	assert.Equal(t, expectedcontentsListResponse, contentsListResponse)

	// ref is the branch we created above  in setup
	ref = newBranch
	refType = "branch"
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/files?ref=%s", user2.Name, repo1.Name, ref), &filesOptions)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsListResponse)
	assert.NotNil(t, contentsListResponse)
	branchCommit, _ := gitRepo.GetBranchCommit(ref)
	lastCommit, _ = branchCommit.GetCommitByPath("README.md")
	expectedcontentsListResponse = getExpectedcontentsListResponseForFiles(ref, refType, lastCommit.ID.String())
	assert.Equal(t, expectedcontentsListResponse, contentsListResponse)

	// ref is the new tag we created above in setup
	ref = newTag
	refType = "tag"
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/files?ref=%s", user2.Name, repo1.Name, ref), &filesOptions)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsListResponse)
	assert.NotNil(t, contentsListResponse)
	tagCommit, _ := gitRepo.GetTagCommit(ref)
	lastCommit, _ = tagCommit.GetCommitByPath("README.md")
	expectedcontentsListResponse = getExpectedcontentsListResponseForFiles(ref, refType, lastCommit.ID.String())
	assert.Equal(t, expectedcontentsListResponse, contentsListResponse)

	// ref is a commit
	ref = commitID
	refType = "commit"
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/files?ref=%s", user2.Name, repo1.Name, ref), &filesOptions)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsListResponse)
	assert.NotNil(t, contentsListResponse)
	expectedcontentsListResponse = getExpectedcontentsListResponseForFiles(ref, refType, commitID)
	assert.Equal(t, expectedcontentsListResponse, contentsListResponse)

	// Test file contents a file with a bad ref
	ref = "badref"
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/files?ref=%s", user2.Name, repo1.Name, ref), &filesOptions)
	MakeRequest(t, req, http.StatusNotFound)

	// Test accessing private ref with user token that does not have access - should fail
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/files", user2.Name, repo16.Name), &filesOptions).
		AddTokenAuth(token4)
	MakeRequest(t, req, http.StatusNotFound)

	// Test access private ref of owner of token
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/files", user2.Name, repo16.Name), &filesOptions).
		AddTokenAuth(token2)
	MakeRequest(t, req, http.StatusOK)

	// Test access of org org3 private repo file by owner user2
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/files", org3.Name, repo3.Name), &filesOptions).
		AddTokenAuth(token2)
	MakeRequest(t, req, http.StatusOK)
}
