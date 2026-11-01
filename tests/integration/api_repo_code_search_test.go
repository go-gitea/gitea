// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	code_indexer "gitea.dev/modules/indexer/code"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
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

	t.Run("Query", func(t *testing.T) {
		const prToUpdateCommit = "62fb502a7172d4453f0322a2cc85bddffa57f07a"
		for _, c := range []struct {
			query    string
			expected []string
		}{
			{"q=WoW&ref=" + prToUpdateCommit, []string{"File-WoW"}},
			{"q=/Desc.*repo1/", []string{"README.md"}},
			{"q=Description+path:docs&ref=sub-home-md-img-check", []string{"docs/README.md"}},
		} {
			assert.Equal(t, c.expected, searchPaths(t, "/api/v1/repos/user2/repo1/code/search?"+c.query), c.query)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		for _, query := range []string{"q=WoW+language:go", "q=WoW+repo:user2/repo1", "q=%22unterminated", "q=Description&page=334&limit=3"} {
			search(t, "/api/v1/repos/user2/repo1/code/search?"+query, "", http.StatusUnprocessableEntity)
		}
		search(t, "/api/v1/repos/user2/repo1/code/search?q=WoW&ref=no-such-ref", "", http.StatusNotFound)
	})

	t.Run("EmptyRepo", func(t *testing.T) {
		MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{Name: "empty-code-search"}).AddTokenAuth(getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)), http.StatusCreated)
		search(t, "/api/v1/repos/user2/empty-code-search/code/search?q=Description&ref=no-such-ref", "", http.StatusNotFound)
		res := search(t, "/api/v1/repos/user2/empty-code-search/code/search?q=Description", "", http.StatusOK)
		assert.Zero(t, res.TotalCount)
		assert.False(t, res.IncompleteResults)
		assert.Equal(t, []*api.CodeSearchResultItem{}, res.Items)
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

func TestAPISearchCode(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
	repo16 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo16"})
	code_indexer.UpdateRepoIndexer(repo1)
	code_indexer.UpdateRepoIndexer(repo16)
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)

	searchPaths := func(t *testing.T, query, token string) (paths []string) {
		t.Helper()
		resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/search/code?"+query).AddTokenAuth(token), http.StatusOK)
		for _, item := range DecodeJSON(t, resp, &api.CodeSearchResults{}).Items {
			paths = append(paths, item.Repository.FullName+"/"+item.Path)
		}
		return paths
	}

	t.Run("Result", func(t *testing.T) {
		resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/search/code?q=Description").AddTokenAuth(token), http.StatusOK)
		res := DecodeJSON(t, resp, &api.CodeSearchResults{})
		assert.EqualValues(t, 1, res.TotalCount)
		require.Len(t, res.Items, 1)
		assert.Equal(t, "user2/repo1", res.Items[0].Repository.FullName)
		assert.Equal(t, "README.md", res.Items[0].Path)
	})

	t.Run("Query", func(t *testing.T) {
		for _, c := range []struct {
			query    string
			expected []string
		}{
			{"q=Description+repo:User2/Repo1", []string{"user2/repo1/README.md"}},
			{"q=Description+org:org3", nil},
			{"q=Description+repo:nobody/missing+user:user2", []string{"user2/repo1/README.md"}},
			{"q=Description+path:docs", nil},
			{"q=Description+language:markdown", []string{"user2/repo1/README.md"}},
		} {
			assert.Equal(t, c.expected, searchPaths(t, c.query, token), c.query)
		}
	})

	t.Run("Access", func(t *testing.T) {
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/search/code?q=signed"), http.StatusUnauthorized)
		assert.Equal(t, []string{"user2/repo16/readme.md"}, searchPaths(t, "q=signed", getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)))
		assert.Nil(t, searchPaths(t, "q=signed", getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)))
		assert.Equal(t, []string{"user2/repo16/readme.md"}, searchPaths(t, "q=signed", getUserToken(t, "user1", auth_model.AccessTokenScopeReadRepository)))
	})

	t.Run("Invalid", func(t *testing.T) {
		for _, query := range []string{"q=/Description/", "q=%22unterminated", "q=x&page=334&limit=3"} {
			MakeRequest(t, NewRequest(t, "GET", "/api/v1/search/code?"+query).AddTokenAuth(token), http.StatusUnprocessableEntity)
		}
	})

	t.Run("Disabled", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Service.Explore.DisableCodePage, true)()
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/search/code?q=Description").AddTokenAuth(token), http.StatusNotFound)
	})

	t.Run("CodeUnitDisabled", func(t *testing.T) {
		disabledUnits := unit.DisabledRepoUnitsGet()
		unit.DisabledRepoUnitsSet(append([]unit.Type{unit.TypeCode}, disabledUnits...))
		defer unit.DisabledRepoUnitsSet(disabledUnits)
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/search/code?q=Description").AddTokenAuth(token), http.StatusNotFound)
	})

	t.Run("TwoFactorRequired", func(t *testing.T) {
		defer test.MockVariableValue(&setting.TwoFactorAuthEnforced, true)()
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/search/code?q=Description").AddTokenAuth(token), http.StatusForbidden)
	})
}
