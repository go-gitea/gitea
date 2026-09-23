// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRepoSearchCode(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	search := func(t *testing.T, url, token string, expectedStatus int) *api.CodeSearchResults {
		t.Helper()
		resp := MakeRequest(t, NewRequest(t, "GET", url).AddTokenAuth(token), expectedStatus)
		if expectedStatus != http.StatusOK {
			return nil
		}
		return DecodeJSON(t, resp, &api.CodeSearchResults{})
	}
	searchPaths := func(t *testing.T, url string) (paths []string) {
		t.Helper()
		for _, item := range search(t, url, "", http.StatusOK).Items {
			paths = append(paths, item.Path)
		}
		return paths
	}

	t.Run("Result", func(t *testing.T) {
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		commitID, err := git.GetBranchCommitID(t.Context(), repo1, "master")
		require.NoError(t, err)

		res := search(t, "/api/v1/repos/user2/repo1/code/search?q=Description", "", http.StatusOK)
		assert.EqualValues(t, 1, res.TotalCount)
		assert.False(t, res.IncompleteResults)
		require.Len(t, res.Items, 1)
		item := res.Items[0]
		assert.Equal(t, "README.md", item.Name)
		assert.Equal(t, "README.md", item.Path)
		assert.Equal(t, repo1.ID, item.Repository.ID)
		assert.Equal(t, setting.AppURL+"api/v1/repos/user2/repo1/contents/README.md?ref="+commitID, item.URL)
		assert.Equal(t, setting.AppURL+"user2/repo1/src/commit/"+commitID+"/README.md", item.HTMLURL)
		assert.Equal(t, []string{"2", "3"}, item.LineNumbers)
		assert.Equal(t, []*api.CodeSearchTextMatch{{
			ObjectURL:  item.URL,
			ObjectType: "FileContent",
			Property:   "content",
			Fragment:   "\nDescription for repo1",
			Matches:    []*api.CodeSearchTextMatchTerm{{Text: "Description", Indices: []int{1, 12}}},
		}}, item.TextMatches)
	})

	t.Run("Query", func(t *testing.T) {
		const prToUpdateCommit = "62fb502a7172d4453f0322a2cc85bddffa57f07a" // File-WoW only exists in this commit
		for _, c := range []struct {
			query    string
			expected []string
		}{
			{"q=WoW", nil},
			{"q=WoW&ref=" + prToUpdateCommit, []string{"File-WoW"}},
			{"q=description", nil},
			{"q=description&search_mode=words", []string{"README.md"}},
			{"q=Desc.*repo1&search_mode=regexp", []string{"README.md"}},
			{"q=Description&path=/", []string{"README.md"}},
			{"q=Description&path=docs", nil},
			{"q=Description&ref=sub-home-md-img-check&path=docs", []string{"docs/README.md"}},
			{"q=Description&ref=sub-home-md-img-check&path=doc", nil},
		} {
			assert.Equal(t, c.expected, searchPaths(t, "/api/v1/repos/user2/repo1/code/search?"+c.query), c.query)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		search(t, "/api/v1/repos/user2/repo1/code/search", "", http.StatusUnprocessableEntity)
		search(t, "/api/v1/repos/user2/repo1/code/search?q=WoW&search_mode=fuzzy", "", http.StatusUnprocessableEntity)
		search(t, "/api/v1/repos/user2/repo1/code/search?q=WoW&ref=no-such-ref", "", http.StatusNotFound)
	})

	t.Run("PrivateRepo", func(t *testing.T) {
		const url = "/api/v1/repos/user2/repo16/code/search?q=signed"
		search(t, url, "", http.StatusNotFound)
		search(t, url, getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly), http.StatusNotFound)
		res := search(t, url, getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository), http.StatusOK)
		require.Len(t, res.Items, 1)
		assert.Equal(t, "readme.md", res.Items[0].Path)
	})
}
