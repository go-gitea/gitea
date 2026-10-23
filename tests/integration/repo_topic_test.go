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

	// search all topics
	res := MakeRequest(t, NewRequest(t, "GET", searchURL.String()), http.StatusOK)
	topics := DecodeJSON(t, res, &struct {
		TopicNames []*api.TopicResponse `json:"topics"`
	}{})
	assert.Len(t, topics.TopicNames, 6)
	assert.Equal(t, "6", res.Header().Get("x-total-count"))

	// pagination search topics
	topics.TopicNames = nil
	query := url.Values{"page": []string{"1"}, "limit": []string{"4"}}

	searchURL.RawQuery = query.Encode()
	res = MakeRequest(t, NewRequest(t, "GET", searchURL.String()), http.StatusOK)
	topics = DecodeJSON(t, res, &struct {
		TopicNames []*api.TopicResponse `json:"topics"`
	}{})
	assert.Len(t, topics.TopicNames, 4)
	assert.Equal(t, "6", res.Header().Get("x-total-count"))

	// second page
	topics.TopicNames = nil
	query = url.Values{"page": []string{"2"}, "limit": []string{"4"}}

	searchURL.RawQuery = query.Encode()
	res = MakeRequest(t, NewRequest(t, "GET", searchURL.String()), http.StatusOK)
	topics = DecodeJSON(t, res, &struct {
		TopicNames []*api.TopicResponse `json:"topics"`
	}{})
	assert.Len(t, topics.TopicNames, 2)
	assert.Equal(t, "6", res.Header().Get("x-total-count"))

	// add keyword search
	topics.TopicNames = nil
	query = url.Values{"page": []string{"1"}, "limit": []string{"4"}}
	query.Add("q", "topic")
	searchURL.RawQuery = query.Encode()
	res = MakeRequest(t, NewRequest(t, "GET", searchURL.String()), http.StatusOK)
	topics = DecodeJSON(t, res, &struct {
		TopicNames []*api.TopicResponse `json:"topics"`
	}{})
	assert.Len(t, topics.TopicNames, 2)

	topics.TopicNames = nil
	query.Set("q", "database")
	searchURL.RawQuery = query.Encode()
	res = MakeRequest(t, NewRequest(t, "GET", searchURL.String()), http.StatusOK)
	topics = DecodeJSON(t, res, &struct {
		TopicNames []*api.TopicResponse `json:"topics"`
	}{})
	if assert.Len(t, topics.TopicNames, 1) {
		assert.EqualValues(t, 2, topics.TopicNames[0].ID)
		assert.Equal(t, "database", topics.TopicNames[0].Name)
		assert.Equal(t, 1, topics.TopicNames[0].RepoCount)
	}
}
