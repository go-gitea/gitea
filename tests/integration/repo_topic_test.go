// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestRepoTopicsPostRejectsCrossOrigin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	req := NewRequestWithValues(t, "POST", "/user2/repo1/topics", map[string]string{"topics": "cross"}).
		SetHeader("Sec-Fetch-Site", "cross-site")
	session.MakeRequest(t, req, http.StatusForbidden)

	req = NewRequestWithValues(t, "POST", "/user2/repo1/topics", map[string]string{"topics": "same"}).
		SetHeader("Sec-Fetch-Site", "same-origin")
	session.MakeRequest(t, req, http.StatusOK)
}

func TestTopicSearch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	searchURL, _ := url.Parse("/explore/topics/search")

	for _, testCase := range []struct {
		query url.Values
		count int
	}{
		// search all topics
		{nil, 6},
		// pagination search topics
		{url.Values{"page": {"1"}, "limit": {"4"}}, 4},
		// second page
		{url.Values{"page": {"2"}, "limit": {"4"}}, 2},
		// add keyword search
		{url.Values{"page": {"1"}, "limit": {"4"}, "q": {"topic"}}, 2},
		{url.Values{"page": {"1"}, "limit": {"4"}, "q": {"database"}}, 1},
	} {
		searchURL.RawQuery = testCase.query.Encode()
		res := MakeRequest(t, NewRequest(t, "GET", searchURL.String()), http.StatusOK)
		topics := DecodeJSON(t, res, &struct {
			TopicNames []*api.TopicResponse `json:"topics"`
		}{})
		if assert.Len(t, topics.TopicNames, testCase.count) && testCase.query.Get("q") == "database" {
			assert.EqualValues(t, 2, topics.TopicNames[0].ID)
			assert.Equal(t, "database", topics.TopicNames[0].Name)
			assert.Equal(t, 1, topics.TopicNames[0].RepoCount)
		}
		if testCase.query.Get("q") == "" {
			assert.Equal(t, "6", res.Header().Get("x-total-count"))
		}
	}
}
