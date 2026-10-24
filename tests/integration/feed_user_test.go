// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/xml"
	"net/http"
	"strings"
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
	defer tests.PrepareTestEnv(t)()

	resp := MakeRequest(t, NewRequest(t, "GET", "/user2.atom"), http.StatusOK)
	assert.Contains(t, resp.Body.String(), `<feed xmlns="http://www.w3.org/2005/Atom"`)

	resp = MakeRequest(t, NewRequest(t, "GET", "/user2.rss"), http.StatusOK)
	assert.Contains(t, resp.Body.String(), `<rss version="2.0"`)
	var rss RSS
	assert.NoError(t, xml.Unmarshal(resp.Body.Bytes(), &rss))
	assert.Contains(t, rss.Channel.Link, "/user2")
	assert.NotEmpty(t, rss.Channel.PubDate)

	session := loginUser(t, "user2")
	readUser := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)
	for _, tc := range []struct {
		link, accept, token, privateMarker string
		visible                            bool
	}{
		{"/user2.rss", "", readUser, "user2/repo2", true},
		{"/user2", "application/rss+xml", readUser, "user2/repo2", true},
		{"/user2.rss", "", getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopePublicOnly), "user2/repo2", false},
		{"/org3.rss", "", getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadOrganization), "org3/repo3", true},
		{"/user2?tab=activity", "", getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeAll), "user2/repo2", false},
	} {
		req := NewRequest(t, "GET", tc.link).SetHeader("Accept", tc.accept).AddBasicAuth("user2", tc.token)
		assert.Equal(t, tc.visible, strings.Contains(MakeRequest(t, req, http.StatusOK).Body.String(), tc.privateMarker), tc.link)
	}
	readPackage := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadPackage)
	for _, link := range []string{"/user2.rss", "/org3.rss"} {
		MakeRequest(t, NewRequest(t, "GET", link).AddBasicAuth("user2", readPackage), http.StatusForbidden)
	}
}
