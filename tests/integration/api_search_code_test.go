// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	code_indexer "gitea.dev/modules/indexer/code"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPISearchCodeNotLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "repo1")
	assert.NoError(t, err)
	code_indexer.UpdateRepoIndexer(repo)

	// test with no keyword
	req := NewRequest(t, "GET", "/api/v1/search/code")
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	req = NewRequest(t, "GET", "/api/v1/search/code?q=Description")
	resp := MakeRequest(t, req, http.StatusOK)

	var apiCodeSearchResults api.CodeSearchResults
	DecodeJSON(t, resp, &apiCodeSearchResults)
	assert.Equal(t, int64(1), apiCodeSearchResults.TotalCount)
	assert.Len(t, apiCodeSearchResults.Items, 1)
	assert.Equal(t, "README.md", apiCodeSearchResults.Items[0].Name)
	assert.Equal(t, "README.md", apiCodeSearchResults.Items[0].Path)
	assert.Equal(t, "Markdown", apiCodeSearchResults.Items[0].Language)
	assert.Len(t, apiCodeSearchResults.Items[0].Lines, 2)
	assert.Equal(t, "\n", apiCodeSearchResults.Items[0].Lines[0].RawContent)
	assert.Equal(t, "Description for repo1", apiCodeSearchResults.Items[0].Lines[1].RawContent)

	unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.NotEmpty(t, apiCodeSearchResults.Items[0].Sha)

	assert.Equal(t, setting.AppURL+"api/v1/repos/user2/repo1/contents/README.md?ref="+apiCodeSearchResults.Items[0].Sha, apiCodeSearchResults.Items[0].URL)
	assert.Equal(t, setting.AppURL+"user2/repo1/blob/"+apiCodeSearchResults.Items[0].Sha+"/README.md", apiCodeSearchResults.Items[0].HTMLURL)

	assert.Equal(t, int64(1), apiCodeSearchResults.Items[0].Repository.ID)

	assert.Len(t, apiCodeSearchResults.Languages, 1)
	assert.Equal(t, "Markdown", apiCodeSearchResults.Languages[0].Language)
	assert.Equal(t, 1, apiCodeSearchResults.Languages[0].Count)
}

func TestAPISearchCodeRepoFilter(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo1, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "repo1")
	assert.NoError(t, err)
	repo2, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "repo2")
	assert.NoError(t, err)
	code_indexer.UpdateRepoIndexer(repo1)
	code_indexer.UpdateRepoIndexer(repo2)

	req := NewRequest(t, "GET", "/api/v1/search/code?q=Description&repo=user2/repo1")
	resp := MakeRequest(t, req, http.StatusOK)
	var results api.CodeSearchResults
	DecodeJSON(t, resp, &results)
	assert.Equal(t, int64(1), results.TotalCount)
	assert.Len(t, results.Items, 1)
	assert.Equal(t, "README.md", results.Items[0].Path)

	req = NewRequest(t, "GET", "/api/v1/search/code?q=Description&repo=user2/repo2")
	resp = MakeRequest(t, req, http.StatusOK)
	results = api.CodeSearchResults{}
	DecodeJSON(t, resp, &results)
	assert.Zero(t, results.TotalCount)
	assert.Empty(t, results.Items)

	req = NewRequest(t, "GET", "/api/v1/search/code?q=Description&repo=user2/repo1&repo=user2/repo2")
	resp = MakeRequest(t, req, http.StatusOK)
	results = api.CodeSearchResults{}
	DecodeJSON(t, resp, &results)
	assert.Equal(t, int64(1), results.TotalCount)
	assert.Len(t, results.Items, 1)

	req = NewRequest(t, "GET", "/api/v1/search/code?q=Description&repo=invalid")
	resp = MakeRequest(t, req, http.StatusOK)
	results = api.CodeSearchResults{}
	DecodeJSON(t, resp, &results)
	assert.Zero(t, results.TotalCount)
	assert.Empty(t, results.Items)
}

func TestAPISearchCodeAdminRepoFilter(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo1, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "repo1")
	assert.NoError(t, err)
	repo2, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "repo2")
	assert.NoError(t, err)
	code_indexer.UpdateRepoIndexer(repo1)
	code_indexer.UpdateRepoIndexer(repo2)

	token := getUserToken(t, "user1", auth_model.AccessTokenScopeReadRepository)
	req := NewRequest(t, "GET", "/api/v1/search/code?q=Description&repo=user2/repo2").
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var results api.CodeSearchResults
	DecodeJSON(t, resp, &results)
	assert.Zero(t, results.TotalCount)
	assert.Empty(t, results.Items)
}

func TestAPISearchCodePublicOnly(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo1, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "repo1")
	assert.NoError(t, err)
	repo2, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "repo2")
	assert.NoError(t, err)
	code_indexer.UpdateRepoIndexer(repo1)
	code_indexer.UpdateRepoIndexer(repo2)

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)
	req := NewRequest(t, "GET", "/api/v1/search/code?q=home+page").
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var results api.CodeSearchResults
	DecodeJSON(t, resp, &results)
	assert.Equal(t, int64(1), results.TotalCount)
	assert.Len(t, results.Items, 1)
	assert.Equal(t, "Home.md", results.Items[0].Path)

	publicOnlyToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)
	req = NewRequest(t, "GET", "/api/v1/search/code?q=home+page").
		AddTokenAuth(publicOnlyToken)
	resp = MakeRequest(t, req, http.StatusOK)

	results = api.CodeSearchResults{}
	DecodeJSON(t, resp, &results)
	assert.Zero(t, results.TotalCount)
	assert.Empty(t, results.Items)
}
