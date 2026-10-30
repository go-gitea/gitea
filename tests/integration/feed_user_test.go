// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/xml"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RSS is a struct to unmarshal RSS feeds test only
type RSS struct {
	Channel struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		Description string `xml:"description"`
		PubDate     string `xml:"pubDate"`
		Items       []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
			PubDate     string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
}

func TestFeedUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("Repo", func(t *testing.T) {
		t.Run("RSS", func(t *testing.T) {
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
	})

	t.Run("User", func(t *testing.T) {
		t.Run("Atom", func(t *testing.T) {
			req := NewRequest(t, "GET", "/user2.atom")
			resp := MakeRequest(t, req, http.StatusOK)

			data := resp.Body.String()
			assert.Contains(t, data, `<feed xmlns="http://www.w3.org/2005/Atom"`)
		})

		t.Run("RSS", func(t *testing.T) {
			req := NewRequest(t, "GET", "/user2.rss")
			resp := MakeRequest(t, req, http.StatusOK)

			data := resp.Body.String()
			assert.Contains(t, data, `<rss version="2.0"`)

			var rss RSS
			err := xml.Unmarshal(resp.Body.Bytes(), &rss)
			assert.NoError(t, err)
			assert.Contains(t, rss.Channel.Link, "/user2")
			assert.NotEmpty(t, rss.Channel.PubDate)
		})
	})

	// TestFeedRepoContentTokenScopes ensures repository feed endpoints enforce the
	// repository token scope, so a PAT without repository scope cannot read private
	// repository commit/activity data through RSS/Atom feeds.
	t.Run("RepoContentTokenScopes", func(t *testing.T) {
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
	})

	// TestFeedUserPublicOnlyToken ensures a public-only API token cannot surface a user's
	// private activity through their profile feed, even when authenticated as the owner.
	t.Run("PublicOnlyToken", func(t *testing.T) {
		fullToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser)
		publicOnlyToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopePublicOnly)

		for _, requestURL := range []string{"/user2.rss", "/user2?tab=activity"} {
			t.Run(requestURL, func(t *testing.T) {
				// user2 has activity on the private repo user2/repo2
				const privateMarker = "user2/repo2"

				// a normal read:user token authenticated as the owner sees private activity
				reqFull := NewRequest(t, "GET", requestURL)
				reqFull.SetBasicAuth("user2", fullToken)
				respFull := MakeRequest(t, reqFull, http.StatusOK)
				assert.Contains(t, respFull.Body.String(), privateMarker)

				// a public-only token must not surface the private activity
				reqPublicOnly := NewRequest(t, "GET", requestURL)
				reqPublicOnly.SetBasicAuth("user2", publicOnlyToken)
				respPublicOnly := MakeRequest(t, reqPublicOnly, http.StatusOK)
				assert.NotContains(t, respPublicOnly.Body.String(), privateMarker)
			})
		}
	})

	t.Run("RepoTagsWithoutReleasesAccess", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		require.NoError(t, repo_service.UpdateRepositoryUnits(t.Context(), repo, nil, []unit_model.Type{unit_model.TypeReleases}))

		resp := MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/tags.rss"), http.StatusOK)
		data := resp.Body.String()
		assert.Contains(t, data, "/user2/repo1/src/tag/v1.0")
		assert.NotContains(t, data, "pre-release")
		assert.NotContains(t, data, "some text for a pre release")
	})

	t.Run("RepoBranchFeedWithoutCodeAccess", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		require.NoError(t, repo_service.UpdateRepositoryUnits(t.Context(), repo, nil, []unit_model.Type{unit_model.TypeCode}))
		MakeRequest(t, NewRequest(t, "GET", "/user2/repo1").SetHeader("Accept", "application/rss+xml"), http.StatusNotFound)
		MakeRequest(t, NewRequest(t, "GET", "/user2/repo1").SetHeader("Accept", "application/atom+xml"), http.StatusNotFound)
		MakeRequest(t, NewRequest(t, "GET", "/user2/repo1.rss"), http.StatusOK)
	})
}
