// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/xml"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestFeedRepo(t *testing.T) {
	t.Run("RSS", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()

		req := NewRequest(t, "GET", "/user2/repo1.rss")
		resp := MakeRequest(t, req, http.StatusOK)

		data := resp.Body.String()
		assert.Contains(t, data, `<rss version="2.0"`)

		var rss RSS
		err := xml.Unmarshal(resp.Body.Bytes(), &rss)
		assert.NoError(t, err)
		assert.Contains(t, rss.Channel.Link, "/user2/repo1")
		assert.NotEmpty(t, rss.Channel.PubDate)
		assert.Len(t, rss.Channel.Items, 1)
		assert.Equal(t, "issue5", rss.Channel.Items[0].Description)
		assert.NotEmpty(t, rss.Channel.Items[0].PubDate)
	})
}

// TestFeedRepoContentTokenScopes ensures repository feed endpoints enforce the
// repository token scope, so a PAT without repository scope cannot read private
// repository commit/activity data through RSS/Atom feeds.
func TestFeedRepoContentTokenScopes(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user2/repo2 is a private repository owned by user2
	ownerReadToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)
	miscToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadMisc)

	urls := []string{
		"/user2/repo2.rss",
		"/user2/repo2.atom",
		"/user2/repo2/rss/branch/master",
		"/user2/repo2/atom/branch/master",
		"/user2/repo2/rss/branch/master/README.md",
		"/user2/repo2/tags.rss",
		"/user2/repo2/tags.atom",
		"/user2/repo2/releases.rss",
		"/user2/repo2/releases.atom",
	}

	for _, url := range urls {
		t.Run(url, func(t *testing.T) {
			// feed routes only accept basic auth, so authenticate as the advisory PoC does (user:token)
			reqDenied := NewRequest(t, "GET", url)
			reqDenied.SetBasicAuth("user2", miscToken)
			// a token without repository scope must be denied
			MakeRequest(t, reqDenied, http.StatusForbidden)

			reqAllowed := NewRequest(t, "GET", url)
			reqAllowed.SetBasicAuth("user2", ownerReadToken)
			// a token with repository read scope is allowed
			MakeRequest(t, reqAllowed, http.StatusOK)
		})
	}
}
