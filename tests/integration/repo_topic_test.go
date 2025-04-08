// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestTopicSearch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	searchURL, _ := url.Parse("/explore/topics/search")
	var topics struct {
		TopicNames []*api.TopicResponse `json:"topics"`
	}

	// search all topics
	res := MakeRequest(t, NewRequest(t, "GET", searchURL.String()), http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.Len(t, topics.TopicNames, 6)
	assert.Equal(t, "6", res.Header().Get("x-total-count"))

	// pagination search topics
	topics.TopicNames = nil
	query := url.Values{"page": []string{"1"}, "limit": []string{"4"}}

	searchURL.RawQuery = query.Encode()
	res = MakeRequest(t, NewRequest(t, "GET", searchURL.String()), http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.Len(t, topics.TopicNames, 4)
	assert.Equal(t, "6", res.Header().Get("x-total-count"))

	// second page
	topics.TopicNames = nil
	query = url.Values{"page": []string{"2"}, "limit": []string{"4"}}

	searchURL.RawQuery = query.Encode()
	res = MakeRequest(t, NewRequest(t, "GET", searchURL.String()), http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.Len(t, topics.TopicNames, 2)
	assert.Equal(t, "6", res.Header().Get("x-total-count"))

	// add keyword search
	topics.TopicNames = nil
	query = url.Values{"page": []string{"1"}, "limit": []string{"4"}}
	query.Add("q", "topic")
	searchURL.RawQuery = query.Encode()
	res = MakeRequest(t, NewRequest(t, "GET", searchURL.String()), http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.Len(t, topics.TopicNames, 2)

	topics.TopicNames = nil
	query.Set("q", "database")
	searchURL.RawQuery = query.Encode()
	res = MakeRequest(t, NewRequest(t, "GET", searchURL.String()), http.StatusOK)
	DecodeJSON(t, res, &topics)
	if assert.Len(t, topics.TopicNames, 1) {
		assert.EqualValues(t, 2, topics.TopicNames[0].ID)
		assert.Equal(t, "database", topics.TopicNames[0].Name)
		assert.Equal(t, 1, topics.TopicNames[0].RepoCount)
	}
}
