// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"io"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	repo_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getExpectedContentsResponseForContents(inputRef string, expectedRef git.RefName, lastCommitSHA string) *api.ContentsResponse {
	treePath := "README.md"
	selfURL := setting.AppURL + "api/v1/repos/user2/repo1/contents/" + treePath + "?ref=" + url.QueryEscape(inputRef)
	htmlURL := setting.AppURL + "user2/repo1/src/" + expectedRef.RefWebLinkPath() + "/" + treePath
	gitURL := setting.AppURL + "api/v1/repos/user2/repo1/git/blobs/4b4851ad51df6a7d9f25c979345979eaeb5b349f"
	return &api.ContentsResponse{
		Name:              treePath,
		Path:              treePath,
		SHA:               "4b4851ad51df6a7d9f25c979345979eaeb5b349f",
		LastCommitSHA:     new(lastCommitSHA),
		LastCommitterDate: new(time.Date(2017, time.March, 19, 16, 47, 59, 0, time.FixedZone("", -14400))),
		LastAuthorDate:    new(time.Date(2017, time.March, 19, 16, 47, 59, 0, time.FixedZone("", -14400))),
		Type:              "file",
		Mode:              "100644",
		Size:              30,
		Encoding:          new("base64"),
		Content:           new("IyByZXBvMQoKRGVzY3JpcHRpb24gZm9yIHJlcG8x"),
		URL:               &selfURL,
		HTMLURL:           &htmlURL,
		GitURL:            &gitURL,
		DownloadURL:       new(setting.AppURL + "user2/repo1/raw/" + expectedRef.RefWebLinkPath() + "/" + treePath),
		Links: &api.FileLinksResponse{
			Self:    &selfURL,
			GitURL:  &gitURL,
			HTMLURL: &htmlURL,
		},
	}
}

func TestAPIGetContents(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		testAPIGetContentsRefFormats(t)
		testAPIGetContents(t, u)
		testAPIGetContentsExt(t)
	})
}

func testAPIGetContents(t *testing.T, _ *url.URL) {
	/*** SETUP ***/
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})         // owner of the repo1 & repo16
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})          // owner of the repo3, is an org
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})         // owner of neither repos
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})   // public repo
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})   // public repo
	repo16 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16}) // private repo
	treePath := "README.md"

	// Get user2's token
	session := loginUser(t, user2.Name)
	token2 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)
	// Get user4's token
	session = loginUser(t, user4.Name)
	token4 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	// Get the commit ID of the default branch
	gitRepo, err := git.OpenRepository(t.Context(), repo1)
	require.NoError(t, err)
	defer gitRepo.Close()

	defaultBranchCommitID, err := gitRepo.GetBranchCommitID(t.Context(), repo1.DefaultBranch)
	require.NoError(t, err)

	t.Run("FileNotFound", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/no-such/file.md", user2.Name, repo1.Name)
		resp := MakeRequest(t, req, http.StatusNotFound)
		assert.Contains(t, resp.Body.String(), "object does not exist [id: , rel_path: no-such]")
	})

	t.Run("NoRef", func(t *testing.T) {
		ref := git.RefNameFromBranch(repo1.DefaultBranch)
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath)
		resp := MakeRequest(t, req, http.StatusOK)
		contentsResponse := DecodeJSON(t, resp, &api.ContentsResponse{})
		expectedContentsResponse := getExpectedContentsResponseForContents("master", ref, defaultBranchCommitID)
		assert.Equal(t, *expectedContentsResponse, *contentsResponse)
	})

	t.Run("NewlyCreatedBranchSynced", func(t *testing.T) {
		newBranch := "test_branch"
		err = repo_service.CreateNewBranch(t.Context(), user2, repo1, gitRepo, repo1.DefaultBranch, newBranch)
		require.NoError(t, err)

		ref := git.RefNameFromBranch(newBranch)
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref.ShortName())
		resp := MakeRequest(t, req, http.StatusOK)
		contentsResponse := DecodeJSON(t, resp, &api.ContentsResponse{})
		branchCommit, _ := gitRepo.GetBranchCommit(t.Context(), ref.ShortName())
		lastCommit, _ := branchCommit.GetCommitByPath(t.Context(), gitRepo, "README.md")
		expectedContentsResponse := getExpectedContentsResponseForContents(ref.ShortName(), ref, lastCommit.ID.String())
		assert.Equal(t, *expectedContentsResponse, *contentsResponse)
	})

	t.Run("NewlyCreatedTagSynced", func(t *testing.T) {
		newTag := "test_tag"
		err = gitRepo.CreateTag(t.Context(), newTag, defaultBranchCommitID)
		require.NoError(t, err)
		ref := git.RefNameFromTag(newTag)
		tagCommit, _ := gitRepo.GetTagCommit(t.Context(), ref.ShortName())
		lastCommit, _ := tagCommit.GetCommitByPath(t.Context(), gitRepo, "README.md")
		expectedContentsResponse := getExpectedContentsResponseForContents(ref.ShortName(), ref, lastCommit.ID.String())

		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref.ShortName())
		resp := MakeRequest(t, req, http.StatusOK)
		contentsResponse := DecodeJSON(t, resp, &api.ContentsResponse{})
		assert.Equal(t, *expectedContentsResponse, *contentsResponse)
	})

	t.Run("CommitRef", func(t *testing.T) {
		ref := git.RefNameFromCommit(defaultBranchCommitID)
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref.ShortName())
		resp := MakeRequest(t, req, http.StatusOK)
		contentsResponse := DecodeJSON(t, resp, &api.ContentsResponse{})
		expectedContentsResponse := getExpectedContentsResponseForContents(ref.ShortName(), ref, defaultBranchCommitID)
		assert.Equal(t, *expectedContentsResponse, *contentsResponse)
	})

	t.Run("NotExistingRef", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=not-existing", user2.Name, repo1.Name, treePath)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("InternalRef", func(t *testing.T) {
		// Internal pull ref exists in the test repo but must not be accepted via contents API
		assert.True(t, git.IsReferenceExist(t.Context(), gitRepo, "refs/pull/2/head"))
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, url.QueryEscape("refs/pull/2/head"))
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Permission", func(t *testing.T) {
		// Test accessing private ref with user token that does not have access - should fail
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s", user2.Name, repo16.Name, treePath).
			AddTokenAuth(token4)
		MakeRequest(t, req, http.StatusNotFound)

		// Test access private ref of owner of token
		req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/readme.md", user2.Name, repo16.Name).
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusOK)

		// Test access of org org3 private repo file by owner user2
		req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s", org3.Name, repo3.Name, treePath).
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusOK)
	})
}

func testAPIGetContentsRefFormats(t *testing.T) {
	file := "README.md"
	sha := "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	content := "# repo1\n\nDescription for repo1"

	resp := MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo1/raw/"+file), http.StatusOK)
	raw, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, content, string(raw))

	resp = MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo1/raw/"+sha+"/"+file), http.StatusOK)
	raw, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, content, string(raw))

	resp = MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo1/raw/"+file+"?ref="+sha), http.StatusOK)
	raw, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, content, string(raw))

	resp = MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo1/raw/"+file+"?ref=master"), http.StatusOK)
	raw, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, content, string(raw))

	_ = MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo1/raw/docs/README.md?ref=main"), http.StatusNotFound)
	_ = MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo1/raw/README.md?ref=main"), http.StatusOK)
	_ = MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo1/raw/docs/README.md?ref=sub-home-md-img-check"), http.StatusOK)
	_ = MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo1/raw/README.md?ref=sub-home-md-img-check"), http.StatusNotFound)

	// FIXME: this is an incorrect behavior, non-existing branch falls back to default branch
	_ = MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo1/raw/README.md?ref=no-such"), http.StatusOK)
}

func testAPIGetContentsExt(t *testing.T) {
	session := loginUser(t, "user2")
	token2 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	t.Run("DirContents", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/contents-ext?ref=sub-home-md-img-check")
		resp := MakeRequest(t, req, http.StatusOK)
		contentsResponse := DecodeJSON(t, resp, &api.ContentsExtResponse{})
		assert.Nil(t, contentsResponse.FileContents)
		assert.NotNil(t, contentsResponse.DirContents)

		req = NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/contents-ext/.?ref=sub-home-md-img-check")
		resp = MakeRequest(t, req, http.StatusOK)
		contentsResponse = DecodeJSON(t, resp, &api.ContentsExtResponse{})
		assert.Nil(t, contentsResponse.FileContents)
		assert.NotNil(t, contentsResponse.DirContents)

		req = NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/contents-ext/docs?ref=sub-home-md-img-check")
		resp = MakeRequest(t, req, http.StatusOK)
		contentsResponse = DecodeJSON(t, resp, &api.ContentsExtResponse{})
		assert.Nil(t, contentsResponse.FileContents)
		assert.Equal(t, "README.md", contentsResponse.DirContents[0].Name)
		assert.Nil(t, contentsResponse.DirContents[0].Encoding)
		assert.Nil(t, contentsResponse.DirContents[0].Content)
		assert.Nil(t, contentsResponse.DirContents[0].LastCommitSHA)
		assert.Nil(t, contentsResponse.DirContents[0].LastCommitMessage)

		// "includes=file_content" shouldn't affect directory listing
		req = NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/contents-ext/docs?ref=sub-home-md-img-check&includes=file_content")
		resp = MakeRequest(t, req, http.StatusOK)
		contentsResponse = DecodeJSON(t, resp, &api.ContentsExtResponse{})
		assert.Nil(t, contentsResponse.FileContents)
		assert.Equal(t, "README.md", contentsResponse.DirContents[0].Name)
		assert.Nil(t, contentsResponse.DirContents[0].Encoding)
		assert.Nil(t, contentsResponse.DirContents[0].Content)

		req = NewRequestf(t, "GET", "/api/v1/repos/user2/lfs/contents-ext?includes=file_content,lfs_metadata").AddTokenAuth(token2)
		resp = session.MakeRequest(t, req, http.StatusOK)
		contentsResponse = DecodeJSON(t, resp, &api.ContentsExtResponse{})
		assert.Nil(t, contentsResponse.FileContents)
		respFileIdx := slices.IndexFunc(contentsResponse.DirContents, func(response *api.ContentsResponse) bool { return response.Name == "jpeg.jpg" })
		require.NotEqual(t, -1, respFileIdx)
		respFile := contentsResponse.DirContents[respFileIdx]
		assert.Equal(t, "jpeg.jpg", respFile.Name)
		assert.Nil(t, respFile.Encoding)
		assert.Nil(t, respFile.Content)
		assert.Equal(t, new(int64(107)), respFile.LfsSize)
		assert.Equal(t, new("0b8d8b5f15046343fd32f451df93acc2bdd9e6373be478b968e4cad6b6647351"), respFile.LfsOid)
	})
	t.Run("FileContents", func(t *testing.T) {
		// by default, no file content or commit info is returned
		req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/contents-ext/docs/README.md?ref=sub-home-md-img-check")
		resp := MakeRequest(t, req, http.StatusOK)
		contentsResponse := DecodeJSON(t, resp, &api.ContentsExtResponse{})
		assert.Nil(t, contentsResponse.DirContents)
		assert.Equal(t, "README.md", contentsResponse.FileContents.Name)
		assert.Nil(t, contentsResponse.FileContents.Encoding)
		assert.Nil(t, contentsResponse.FileContents.Content)
		assert.Nil(t, contentsResponse.FileContents.LastCommitSHA)
		assert.Nil(t, contentsResponse.FileContents.LastCommitMessage)

		// file content is only returned when `includes=file_content`
		req = NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/contents-ext/docs/README.md?ref=sub-home-md-img-check&includes=file_content,commit_metadata,commit_message")
		resp = MakeRequest(t, req, http.StatusOK)
		contentsResponse = DecodeJSON(t, resp, &api.ContentsExtResponse{})
		assert.Nil(t, contentsResponse.DirContents)
		assert.Equal(t, "README.md", contentsResponse.FileContents.Name)
		assert.NotNil(t, contentsResponse.FileContents.Encoding)
		assert.NotNil(t, contentsResponse.FileContents.Content)
		assert.Equal(t, "4649299398e4d39a5c09eb4f534df6f1e1eb87cc", *contentsResponse.FileContents.LastCommitSHA)
		assert.Equal(t, "Test how READMEs render images when found in a subfolder\n", *contentsResponse.FileContents.LastCommitMessage)

		req = NewRequestf(t, "GET", "/api/v1/repos/user2/lfs/contents-ext/jpeg.jpg?includes=file_content").AddTokenAuth(token2)
		resp = session.MakeRequest(t, req, http.StatusOK)
		contentsResponse = DecodeJSON(t, resp, &api.ContentsExtResponse{})
		assert.Nil(t, contentsResponse.DirContents)
		assert.NotNil(t, contentsResponse.FileContents)
		respFile := contentsResponse.FileContents
		assert.Equal(t, "jpeg.jpg", respFile.Name)
		assert.NotNil(t, respFile.Encoding)
		assert.NotNil(t, respFile.Content)
		assert.Nil(t, contentsResponse.FileContents.LastCommitSHA)
		assert.Nil(t, contentsResponse.FileContents.LastCommitMessage)
		assert.Equal(t, new(int64(107)), respFile.LfsSize)
		assert.Equal(t, new("0b8d8b5f15046343fd32f451df93acc2bdd9e6373be478b968e4cad6b6647351"), respFile.LfsOid)
	})
}
