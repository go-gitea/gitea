// Copyright 2022 The Gitea Authors. All rights reserved.
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
	t.Run("User", func(t *testing.T) {
		t.Run("Atom", func(t *testing.T) {
			defer tests.PrepareTestEnv(t)()

			req := NewRequest(t, "GET", "/user2.atom")
			resp := MakeRequest(t, req, http.StatusOK)

			data := resp.Body.String()
			assert.Contains(t, data, `<feed xmlns="http://www.w3.org/2005/Atom"`)
		})

		t.Run("RSS", func(t *testing.T) {
			defer tests.PrepareTestEnv(t)()

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
}

// TestFeedUserPublicOnlyToken ensures a public-only API token cannot surface a user's
// private activity through their profile feed, even when authenticated as the owner.
func TestFeedUserPublicOnlyToken(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user2 has activity on the private repo user2/repo2
	const privateMarker = "user2/repo2"

	// a normal read:user token authenticated as the owner sees private activity
	fullToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser)
	reqFull := NewRequest(t, "GET", "/user2.rss")
	reqFull.SetBasicAuth("user2", fullToken)
	respFull := MakeRequest(t, reqFull, http.StatusOK)
	assert.Contains(t, respFull.Body.String(), privateMarker)

	// a public-only token must not surface the private activity
	publicOnlyToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopePublicOnly)
	reqPublicOnly := NewRequest(t, "GET", "/user2.rss")
	reqPublicOnly.SetBasicAuth("user2", publicOnlyToken)
	respPublicOnly := MakeRequest(t, reqPublicOnly, http.StatusOK)
	assert.NotContains(t, respPublicOnly.Body.String(), privateMarker)
}

// TestProfileActivityPublicOnlyToken ensures the HTML profile activity tab does not
// surface a user's private activity to a public-only API token, mirroring the RSS/Atom
// guard. The /{username} route is AllowBasic, so a public-only PAT used as the Basic
// password must still be downgraded even for the feed owner.
func TestProfileActivityPublicOnlyToken(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user2 has activity on the private repo user2/repo2
	const privateMarker = "user2/repo2"

	// a normal read:user token authenticated as the owner sees private activity
	fullToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser)
	reqFull := NewRequest(t, "GET", "/user2?tab=activity")
	reqFull.SetBasicAuth("user2", fullToken)
	respFull := MakeRequest(t, reqFull, http.StatusOK)
	assert.Contains(t, respFull.Body.String(), privateMarker)

	// a public-only token must not surface the private activity
	publicOnlyToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopePublicOnly)
	reqPublicOnly := NewRequest(t, "GET", "/user2?tab=activity")
	reqPublicOnly.SetBasicAuth("user2", publicOnlyToken)
	respPublicOnly := MakeRequest(t, reqPublicOnly, http.StatusOK)
	assert.NotContains(t, respPublicOnly.Body.String(), privateMarker)
}

// TestProfileActivityCollaborator ensures that a collaborator with access to a private
// repository can see activities for that repository on the target user's profile activity tab,
// while users without access cannot see them.
func TestProfileActivityCollaborator(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user16 owns private repo22 (big_test_private_3)
	// user15 is a collaborator on repo22, user4 is not
	const privateRepoMarker = "user16/big_test_private_3"

	// Collaborator user15 viewing user16's activity tab should see privateRepoMarker
	sessionUser15 := loginUser(t, "user15")
	reqCollab := NewRequest(t, "GET", "/user16?tab=activity")
	respCollab := sessionUser15.MakeRequest(t, reqCollab, http.StatusOK)
	assert.Contains(t, respCollab.Body.String(), privateRepoMarker)

	// Non-collaborator user4 viewing user16's activity tab must NOT see privateRepoMarker
	sessionUser4 := loginUser(t, "user4")
	reqNonCollab := NewRequest(t, "GET", "/user16?tab=activity")
	respNonCollab := sessionUser4.MakeRequest(t, reqNonCollab, http.StatusOK)
	assert.NotContains(t, respNonCollab.Body.String(), privateRepoMarker)

	// Anonymous viewer must NOT see privateRepoMarker
	reqAnon := NewRequest(t, "GET", "/user16?tab=activity")
	respAnon := MakeRequest(t, reqAnon, http.StatusOK)
	assert.NotContains(t, respAnon.Body.String(), privateRepoMarker)

	// Collaborator user15 using a public-only token must NOT see privateRepoMarker
	publicOnlyToken := getUserToken(t, "user15", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopePublicOnly)
	reqPublicOnly := NewRequest(t, "GET", "/user16?tab=activity")
	reqPublicOnly.SetBasicAuth("user15", publicOnlyToken)
	respPublicOnly := MakeRequest(t, reqPublicOnly, http.StatusOK)
	assert.NotContains(t, respPublicOnly.Body.String(), privateRepoMarker)
}
