// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestExploreRepos(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/explore/repos?q=TheKeyword&topic=1&language=TheLang")
	resp := MakeRequest(t, req, http.StatusOK)
	respStr := resp.Body.String()

	assert.Contains(t, respStr, `<input type="hidden" name="topic" value="true">`)
	assert.Contains(t, respStr, `<input type="hidden" name="language" value="TheLang">`)
	assert.Contains(t, respStr, `<input type="search" name="q" value="TheKeyword"`)
}

// TestUnauthenticatedCannotSeePrivateRepos verifies that unauthenticated
// requests cannot use ?private=true to expose private repositories on
// explore, user-profile, and org-profile pages.
func TestUnauthenticatedCannotSeePrivateRepos(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Pre-verify that the fixture repos used in this test are actually private.
	// If a fixture change accidentally makes one public, this will fail loudly.
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	assert.True(t, repo2.IsPrivate, "repo2 must be private for this security test to be meaningful")
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	assert.True(t, repo3.IsPrivate, "repo3 must be private for this security test to be meaningful")
	repo5 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 5})
	assert.True(t, repo5.IsPrivate, "repo5 must be private for this security test to be meaningful")

	t.Run("ExplorePage", func(t *testing.T) {
		// Use ?q= to ensure the target repo would rank first if it were returned,
		// eliminating false-negatives caused by pagination.
		req := NewRequest(t, "GET", "/explore/repos?private=true&q=repo2")
		resp := MakeRequest(t, req, http.StatusOK)
		body := resp.Body.String()
		assert.NotContains(t, body, "user2/repo2", "private repo user2/repo2 must not be exposed")

		req = NewRequest(t, "GET", "/explore/repos?private=true&q=repo3")
		resp = MakeRequest(t, req, http.StatusOK)
		body = resp.Body.String()
		assert.NotContains(t, body, "org3/repo3", "private repo org3/repo3 must not be exposed")

		req = NewRequest(t, "GET", "/explore/repos?private=true&q=repo5")
		resp = MakeRequest(t, req, http.StatusOK)
		body = resp.Body.String()
		assert.NotContains(t, body, "org3/repo5", "private repo org3/repo5 must not be exposed")
	})

	t.Run("UserProfilePage", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2?tab=repos&private=true&q=repo2")
		resp := MakeRequest(t, req, http.StatusOK)
		body := resp.Body.String()
		assert.NotContains(t, body, "user2/repo2", "private repo user2/repo2 must not be exposed on user profile page")
	})

	t.Run("OrgProfilePage", func(t *testing.T) {
		req := NewRequest(t, "GET", "/org3?private=true&q=repo3")
		resp := MakeRequest(t, req, http.StatusOK)
		body := resp.Body.String()
		assert.NotContains(t, body, "org3/repo3", "private repo org3/repo3 must not be exposed on org page")

		req = NewRequest(t, "GET", "/org3?private=true&q=repo5")
		resp = MakeRequest(t, req, http.StatusOK)
		body = resp.Body.String()
		assert.NotContains(t, body, "org3/repo5", "private repo org3/repo5 must not be exposed on org page")
	})
}

// TestAPISearchReposIsPrivateUnauthenticated verifies that the API search endpoint
// does not expose private repositories when an unauthenticated client passes ?is_private=true.
func TestAPISearchReposIsPrivateUnauthenticated(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	type searchResponse struct {
		OK   bool             `json:"ok"`
		Data []api.Repository `json:"data"`
	}

	t.Run("NoPrivateReposInAllResults", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/repos/search?is_private=true&limit=50")
		resp := MakeRequest(t, req, http.StatusOK)
		var result searchResponse
		DecodeJSON(t, resp, &result)
		assert.True(t, result.OK, "search response should indicate success")
		for _, repo := range result.Data {
			assert.False(t, repo.Private, "repo %s must not be private in unauthenticated response", repo.Name)
		}
	})

	t.Run("Repo2NotInFilteredResults", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/repos/search?is_private=true&q=repo2&limit=50")
		resp := MakeRequest(t, req, http.StatusOK)
		var result searchResponse
		DecodeJSON(t, resp, &result)
		assert.True(t, result.OK, "search response should indicate success")
		repoNames := make([]string, 0, len(result.Data))
		for _, repo := range result.Data {
			assert.False(t, repo.Private, "repo %s must not be private in unauthenticated response", repo.Name)
			repoNames = append(repoNames, repo.Name)
		}
		assert.NotContains(t, repoNames, "repo2",
			"private repo2 must not appear in unauthenticated ?is_private=true response")
	})
}
