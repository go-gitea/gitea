// Copyright 2023 The Gitea Authors. All rights reserved.
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
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIRepoCodeSearch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	session := loginUser(t, repo.OwnerName)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	executeIndexer(t, repo, code_indexer.UpdateRepoIndexer)

	t.Run("WithoutLanguage", func(t *testing.T) {
		urlStr := fmt.Sprintf("api/v1/repos/%s/%s/code_search?keyword=This&token=%s", repo.OwnerName, repo.Name, token)
		req := NewRequest(t, "GET", urlStr)
		resp := MakeRequest(t, req, http.StatusOK)

		var searchResults api.RepoCodeSearchAPIResponse
		DecodeJSON(t, resp, &searchResults)

		assert.Equal(t, 2, searchResults.Total)
		assert.Len(t, searchResults.SearchResults, 2)
		assert.Len(t, searchResults.SearchResultLanguages, 2)

		assert.Equal(t, "Home.md", searchResults.SearchResults[0].Filename)
		assert.Equal(t, "Markdown", searchResults.SearchResults[0].Language)
		assert.Len(t, searchResults.SearchResults[0].LineNumbers, 3)
		assert.Equal(t, 2, searchResults.SearchResults[0].LineNumbers[0])
		assert.Equal(t, 3, searchResults.SearchResults[0].LineNumbers[1])
		assert.Equal(t, 4, searchResults.SearchResults[0].LineNumbers[2])

		assert.Equal(t, "test.xml", searchResults.SearchResults[1].Filename)
		assert.Equal(t, "XML", searchResults.SearchResults[1].Language)
		assert.Len(t, searchResults.SearchResults[1].LineNumbers, 3)
		assert.Equal(t, 1, searchResults.SearchResults[1].LineNumbers[0])
		assert.Equal(t, 2, searchResults.SearchResults[1].LineNumbers[1])
		assert.Equal(t, 3, searchResults.SearchResults[1].LineNumbers[2])

		assert.Equal(t, "Markdown", searchResults.SearchResultLanguages[0].Language)
		assert.Equal(t, 1, searchResults.SearchResultLanguages[0].Count)

		assert.Equal(t, "XML", searchResults.SearchResultLanguages[1].Language)
		assert.Equal(t, 1, searchResults.SearchResultLanguages[1].Count)
	})

	t.Run("WithLanguage", func(t *testing.T) {
		urlStr := fmt.Sprintf("api/v1/repos/%s/%s/code_search?keyword=This&language=Markdown&token=%s", repo.OwnerName, repo.Name, token)
		req := NewRequest(t, "GET", urlStr)
		resp := MakeRequest(t, req, http.StatusOK)

		var searchResults api.RepoCodeSearchAPIResponse
		DecodeJSON(t, resp, &searchResults)

		assert.Equal(t, 1, searchResults.Total)
		assert.Len(t, searchResults.SearchResults, 1)
		assert.Len(t, searchResults.SearchResultLanguages, 2)

		assert.Equal(t, "Home.md", searchResults.SearchResults[0].Filename)
		assert.Equal(t, "Markdown", searchResults.SearchResults[0].Language)
		assert.Len(t, searchResults.SearchResults[0].LineNumbers, 3)
		assert.Equal(t, 2, searchResults.SearchResults[0].LineNumbers[0])
		assert.Equal(t, 3, searchResults.SearchResults[0].LineNumbers[1])
		assert.Equal(t, 4, searchResults.SearchResults[0].LineNumbers[2])

		assert.Equal(t, "Markdown", searchResults.SearchResultLanguages[0].Language)
		assert.Equal(t, 1, searchResults.SearchResultLanguages[0].Count)

		assert.Equal(t, "XML", searchResults.SearchResultLanguages[1].Language)
		assert.Equal(t, 1, searchResults.SearchResultLanguages[1].Count)
	})

	t.Run("WithoutKeyword", func(t *testing.T) {
		urlStr := fmt.Sprintf("api/v1/repos/%s/%s/code_search?token=%s", repo.OwnerName, repo.Name, token)
		req := NewRequest(t, "GET", urlStr)
		MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("IndexerDisabled", func(t *testing.T) {
		setting.Indexer.RepoIndexerEnabled = false

		urlStr := fmt.Sprintf("api/v1/repos/%s/%s/code_search?keyword=This&token=%s", repo.OwnerName, repo.Name, token)
		req := NewRequest(t, "GET", urlStr)
		MakeRequest(t, req, http.StatusNotImplemented)

		setting.Indexer.RepoIndexerEnabled = true
	})
}

func TestAPIUserCodeSearch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})

	executeIndexer(t, repo, code_indexer.UpdateRepoIndexer)

	urlStr := fmt.Sprintf("api/v1/users/%s/code_search?keyword=readme", repo.OwnerName)
	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)

	var searchResults api.RepoCodeSearchAPIResponse
	DecodeJSON(t, resp, &searchResults)

	assert.Equal(t, 1, searchResults.Total)
	assert.Len(t, searchResults.SearchResults, 1)
	assert.Len(t, searchResults.SearchResultLanguages, 1)

	assert.Equal(t, repo.ID, searchResults.SearchResults[0].RepoID)
	assert.Equal(t, "README.md", searchResults.SearchResults[0].Filename)
	assert.Equal(t, "Markdown", searchResults.SearchResults[0].Language)
	assert.Equal(t, "a readme", searchResults.SearchResults[0].FormattedLines)
	assert.Len(t, searchResults.SearchResults[0].LineNumbers, 2)
	assert.Equal(t, 1, searchResults.SearchResults[0].LineNumbers[0])
	assert.Equal(t, 2, searchResults.SearchResults[0].LineNumbers[1])

	assert.Equal(t, "Markdown", searchResults.SearchResultLanguages[0].Language)
	assert.Equal(t, 1, searchResults.SearchResultLanguages[0].Count)
}

func TestAPIOrgCodeSearch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 46})

	executeIndexer(t, repo, code_indexer.UpdateRepoIndexer)

	urlStr := fmt.Sprintf("api/v1/orgs/%s/code_search?keyword=%s", repo.OwnerName, url.PathEscape("Repo External tracker"))
	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)

	var searchResults api.RepoCodeSearchAPIResponse
	DecodeJSON(t, resp, &searchResults)

	assert.Equal(t, 1, searchResults.Total)
	assert.Len(t, searchResults.SearchResults, 1)
	assert.Len(t, searchResults.SearchResultLanguages, 1)

	assert.Equal(t, repo.ID, searchResults.SearchResults[0].RepoID)
	assert.Equal(t, "README.md", searchResults.SearchResults[0].Filename)
	assert.Equal(t, "Markdown", searchResults.SearchResults[0].Language)
	assert.Len(t, searchResults.SearchResults[0].LineNumbers, 2)
	assert.Equal(t, 1, searchResults.SearchResults[0].LineNumbers[0])
	assert.Equal(t, 2, searchResults.SearchResults[0].LineNumbers[1])

	assert.Equal(t, "Markdown", searchResults.SearchResultLanguages[0].Language)
	assert.Equal(t, 1, searchResults.SearchResultLanguages[0].Count)
}

func TestAPIGlobalCodeSearch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	executeIndexer(t, repo, code_indexer.UpdateRepoIndexer)

	urlStr := fmt.Sprintf("api/v1/code_search?keyword=%s", url.PathEscape("Description for repo1"))
	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)

	var searchResults api.RepoCodeSearchAPIResponse
	DecodeJSON(t, resp, &searchResults)

	assert.Equal(t, 1, searchResults.Total)
	assert.Len(t, searchResults.SearchResults, 1)
	assert.Len(t, searchResults.SearchResultLanguages, 1)

	assert.Equal(t, repo.ID, searchResults.SearchResults[0].RepoID)
	assert.Equal(t, "README.md", searchResults.SearchResults[0].Filename)
	assert.Equal(t, "Markdown", searchResults.SearchResults[0].Language)
	assert.Len(t, searchResults.SearchResults[0].LineNumbers, 2)
	assert.Equal(t, 2, searchResults.SearchResults[0].LineNumbers[0])
	assert.Equal(t, 3, searchResults.SearchResults[0].LineNumbers[1])
	assert.Len(t, searchResults.SearchResults[0].ContentLines, 2)
	assert.Equal(t, "\n", searchResults.SearchResults[0].ContentLines[0])
	assert.Equal(t, "Description for repo1", searchResults.SearchResults[0].ContentLines[1])

	assert.Equal(t, "Markdown", searchResults.SearchResultLanguages[0].Language)
	assert.Equal(t, 1, searchResults.SearchResultLanguages[0].Count)
}
