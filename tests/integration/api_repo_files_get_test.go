// Copyright 2025 The Gitea Authors. All rights reserved.
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
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIGetRequestedFiles(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})         // owner of the repo1 & repo16
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})          // owner of the repo3, is an org
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})         // owner of neither repos
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})   // public repo
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})   // public repo
	repo16 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16}) // private repo

	// Get user2's token
	session := loginUser(t, user2.Name)
	token2 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	// Get user4's token
	session = loginUser(t, user4.Name)
	token4 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	gitRepo, err := gitrepo.OpenRepository(t.Context(), repo1)
	assert.NoError(t, err)
	defer gitRepo.Close()
	lastCommit, _ := gitRepo.GetCommitByPath("README.md")

	requestFiles := func(t *testing.T, url string, files []string, expectedStatusCode ...int) (ret []*api.ContentsResponse) {
		req := NewRequestWithJSON(t, "POST", url, &api.GetFilesOptions{Files: files})
		resp := MakeRequest(t, req, util.OptionalArg(expectedStatusCode, http.StatusOK))
		if resp.Code != http.StatusOK {
			return nil
		}
		DecodeJSON(t, resp, &ret)
		return ret
	}

	t.Run("User2Get", func(t *testing.T) {
		reqBodyOpt := &api.GetFilesOptions{Files: []string{"README.md"}}
		reqBodyParam, _ := json.Marshal(reqBodyOpt)
		req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/file-contents?body="+url.QueryEscape(string(reqBodyParam)))
		resp := MakeRequest(t, req, http.StatusOK)
		var ret []*api.ContentsResponse
		DecodeJSON(t, resp, &ret)
		expected := []*api.ContentsResponse{getExpectedContentsResponseForContents(repo1.DefaultBranch, "branch", lastCommit.ID.String())}
		assert.Equal(t, expected, ret)
	})
	t.Run("User2NoRef", func(t *testing.T) {
		ret := requestFiles(t, "/api/v1/repos/user2/repo1/file-contents", []string{"README.md"})
		expected := []*api.ContentsResponse{getExpectedContentsResponseForContents(repo1.DefaultBranch, "branch", lastCommit.ID.String())}
		assert.Equal(t, expected, ret)
	})
	t.Run("User2RefBranch", func(t *testing.T) {
		ret := requestFiles(t, "/api/v1/repos/user2/repo1/file-contents?ref=master", []string{"README.md"})
		expected := []*api.ContentsResponse{getExpectedContentsResponseForContents(repo1.DefaultBranch, "branch", lastCommit.ID.String())}
		assert.Equal(t, expected, ret)
	})
	t.Run("User2RefTag", func(t *testing.T) {
		ret := requestFiles(t, "/api/v1/repos/user2/repo1/file-contents?ref=v1.1", []string{"README.md"})
		expected := []*api.ContentsResponse{getExpectedContentsResponseForContents("v1.1", "tag", lastCommit.ID.String())}
		assert.Equal(t, expected, ret)
	})
	t.Run("User2RefCommit", func(t *testing.T) {
		ret := requestFiles(t, "/api/v1/repos/user2/repo1/file-contents?ref=65f1bf27bc3bf70f64657658635e66094edbcb4d", []string{"README.md"})
		expected := []*api.ContentsResponse{getExpectedContentsResponseForContents("65f1bf27bc3bf70f64657658635e66094edbcb4d", "commit", lastCommit.ID.String())}
		assert.Equal(t, expected, ret)
	})
	t.Run("User2RefNotExist", func(t *testing.T) {
		ret := requestFiles(t, "/api/v1/repos/user2/repo1/file-contents?ref=not-exist", []string{"README.md"}, http.StatusNotFound)
		assert.Empty(t, ret)
	})

	t.Run("PermissionCheck", func(t *testing.T) {
		filesOptions := &api.GetFilesOptions{Files: []string{"README.md"}}
		// Test accessing private ref with user token that does not have access - should fail
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/file-contents", user2.Name, repo16.Name), &filesOptions).AddTokenAuth(token4)
		MakeRequest(t, req, http.StatusNotFound)
		// Test access private ref of owner of token
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/file-contents", user2.Name, repo16.Name), &filesOptions).AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusOK)
		// Test access of org org3 private repo file by owner user2
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/file-contents", org3.Name, repo3.Name), &filesOptions).AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusOK)
	})

	t.Run("ResponseList", func(t *testing.T) {
		defer test.MockVariableValue(&setting.API.DefaultPagingNum)()
		defer test.MockVariableValue(&setting.API.DefaultMaxBlobSize)()
		defer test.MockVariableValue(&setting.API.DefaultMaxResponseSize)()

		type expected struct {
			Name       string
			HasContent bool
		}
		assertResponse := func(t *testing.T, expected []*expected, ret []*api.ContentsResponse) {
			require.Len(t, ret, len(expected))
			for i, e := range expected {
				if e == nil {
					assert.Nil(t, ret[i], "item %d", i)
					continue
				}
				assert.Equal(t, e.Name, ret[i].Name, "item %d name", i)
				if e.HasContent {
					require.NotNil(t, ret[i].Content, "item %d content", i)
					assert.NotEmpty(t, *ret[i].Content, "item %d content", i)
				} else {
					assert.Nil(t, ret[i].Content, "item %d content", i)
				}
			}
		}

		// repo1 "DefaultBranch" has 2 files: LICENSE (1064 bytes), README.md (30 bytes)
		ret := requestFiles(t, "/api/v1/repos/user2/repo1/file-contents?ref=DefaultBranch", []string{"no-such.txt", "LICENSE", "README.md"})
		assertResponse(t, []*expected{nil, {"LICENSE", true}, {"README.md", true}}, ret)

		// the returned file list is limited by the DefaultPagingNum
		setting.API.DefaultPagingNum = 2
		ret = requestFiles(t, "/api/v1/repos/user2/repo1/file-contents?ref=DefaultBranch", []string{"no-such.txt", "LICENSE", "README.md"})
		assertResponse(t, []*expected{nil, {"LICENSE", true}}, ret)
		setting.API.DefaultPagingNum = 100

		// if a file exceeds the DefaultMaxBlobSize, the content is not returned
		setting.API.DefaultMaxBlobSize = 200
		ret = requestFiles(t, "/api/v1/repos/user2/repo1/file-contents?ref=DefaultBranch", []string{"no-such.txt", "LICENSE", "README.md"})
		assertResponse(t, []*expected{nil, {"LICENSE", false}, {"README.md", true}}, ret)
		setting.API.DefaultMaxBlobSize = 20000

		// if the total response size would exceed the DefaultMaxResponseSize, then the list stops
		setting.API.DefaultMaxResponseSize = ret[1].Size*4/3 + 10
		ret = requestFiles(t, "/api/v1/repos/user2/repo1/file-contents?ref=DefaultBranch", []string{"no-such.txt", "LICENSE", "README.md"})
		assertResponse(t, []*expected{nil, {"LICENSE", true}}, ret)
		setting.API.DefaultMaxBlobSize = 20000
	})
}
