// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

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

	// repo2 is private and owned by user2; repo3/repo5 are private and owned by org3.
	// None of these should appear in responses to unauthenticated ?private=true requests.

	t.Run("ExplorePage", func(t *testing.T) {
		req := NewRequest(t, "GET", "/explore/repos?private=true")
		resp := MakeRequest(t, req, http.StatusOK)
		body := resp.Body.String()
		assert.NotContains(t, body, "user2/repo2", "private repo user2/repo2 must not be exposed")
		assert.NotContains(t, body, "org3/repo3", "private repo org3/repo3 must not be exposed")
		assert.NotContains(t, body, "org3/repo5", "private repo org3/repo5 must not be exposed")
	})

	t.Run("UserProfilePage", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2?tab=repos&private=true")
		resp := MakeRequest(t, req, http.StatusOK)
		body := resp.Body.String()
		assert.NotContains(t, body, "user2/repo2", "private repo user2/repo2 must not be exposed on user profile page")
	})

	t.Run("OrgProfilePage", func(t *testing.T) {
		req := NewRequest(t, "GET", "/org3?private=true")
		resp := MakeRequest(t, req, http.StatusOK)
		body := resp.Body.String()
		assert.NotContains(t, body, "org3/repo3", "private repo org3/repo3 must not be exposed on org page")
		assert.NotContains(t, body, "org3/repo5", "private repo org3/repo5 must not be exposed on org page")
	})
}
