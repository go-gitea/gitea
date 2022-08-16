// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"net/url"
	"testing"

	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestTopicSearch(t *testing.T) {
	defer prepareTestEnv(t)()
	searchURL, _ := url.Parse("/explore/topics/search")
	var topics struct {
		TopicNames []*api.TopicResponse `json:"topics"`
	}

	query := url.Values{"page": []string{"1"}, "limit": []string{"4"}}

	searchURL.RawQuery = query.Encode()
	res := MakeRequest(t, NewRequest(t, "GET", searchURL.String()), http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.Len(t, topics.TopicNames, 4)
	assert.EqualValues(t, "6", res.Header().Get("x-total-count"))

	query.Add("q", "topic")
	searchURL.RawQuery = query.Encode()
	res = MakeRequest(t, NewRequest(t, "GET", searchURL.String()), http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.Len(t, topics.TopicNames, 2)

	query.Set("q", "database")
	searchURL.RawQuery = query.Encode()
	res = MakeRequest(t, NewRequest(t, "GET", searchURL.String()), http.StatusOK)
	DecodeJSON(t, res, &topics)
	if assert.Len(t, topics.TopicNames, 1) {
		assert.EqualValues(t, 2, topics.TopicNames[0].ID)
		assert.EqualValues(t, "database", topics.TopicNames[0].Name)
		assert.EqualValues(t, 1, topics.TopicNames[0].RepoCount)
	}
}
