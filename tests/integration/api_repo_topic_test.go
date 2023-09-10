// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPITopicSearch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	searchURL, _ := url.Parse("/api/v1/topics/search")
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

func TestAPIRepoTopic(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}) // owner of repo2
	user3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3}) // owner of repo3
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}) // write access to repo 3
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})

	// Get user2's token
	token2 := getUserToken(t, user2.Name, auth_model.AccessTokenScopeWriteRepository)

	// Test read topics using login
	url := fmt.Sprintf("/api/v1/repos/%s/%s/topics", user2.Name, repo2.Name)
	req := NewRequest(t, "GET", url+"?token="+token2)
	res := MakeRequest(t, req, http.StatusOK)
	var topics *api.TopicName
	DecodeJSON(t, res, &topics)
	assert.ElementsMatch(t, []string{"topicname1", "topicname2"}, topics.TopicNames)

	// Log out user2
	url = fmt.Sprintf("/api/v1/repos/%s/%s/topics?token=%s", user2.Name, repo2.Name, token2)

	// Test delete a topic
	req = NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/topics/%s?token=%s", user2.Name, repo2.Name, "Topicname1", token2)
	MakeRequest(t, req, http.StatusNoContent)

	// Test add an existing topic
	req = NewRequestf(t, "PUT", "/api/v1/repos/%s/%s/topics/%s?token=%s", user2.Name, repo2.Name, "Golang", token2)
	MakeRequest(t, req, http.StatusNoContent)

	// Test add a topic
	req = NewRequestf(t, "PUT", "/api/v1/repos/%s/%s/topics/%s?token=%s", user2.Name, repo2.Name, "topicName3", token2)
	MakeRequest(t, req, http.StatusNoContent)

	// Test read topics using token
	req = NewRequest(t, "GET", url)
	res = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.ElementsMatch(t, []string{"topicname2", "golang", "topicname3"}, topics.TopicNames)

	// Test replace topics
	newTopics := []string{"   windows ", "   ", "MAC  "}
	req = NewRequestWithJSON(t, "PUT", url, &api.RepoTopicOptions{
		Topics: newTopics,
	})
	MakeRequest(t, req, http.StatusNoContent)
	req = NewRequest(t, "GET", url)
	res = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.ElementsMatch(t, []string{"windows", "mac"}, topics.TopicNames)

	// Test replace topics with something invalid
	newTopics = []string{"topicname1", "topicname2", "topicname!"}
	req = NewRequestWithJSON(t, "PUT", url, &api.RepoTopicOptions{
		Topics: newTopics,
	})
	MakeRequest(t, req, http.StatusUnprocessableEntity)
	req = NewRequest(t, "GET", url)
	res = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.ElementsMatch(t, []string{"windows", "mac"}, topics.TopicNames)

	// Test with some topics multiple times, less than 25 unique
	newTopics = []string{"t1", "t2", "t1", "t3", "t4", "t5", "t6", "t7", "t8", "t9", "t10", "t11", "t12", "t13", "t14", "t15", "t16", "17", "t18", "t19", "t20", "t21", "t22", "t23", "t24", "t25"}
	req = NewRequestWithJSON(t, "PUT", url, &api.RepoTopicOptions{
		Topics: newTopics,
	})
	MakeRequest(t, req, http.StatusNoContent)
	req = NewRequest(t, "GET", url)
	res = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.Len(t, topics.TopicNames, 25)

	// Test writing more topics than allowed
	newTopics = append(newTopics, "t26")
	req = NewRequestWithJSON(t, "PUT", url, &api.RepoTopicOptions{
		Topics: newTopics,
	})
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// Test add a topic when there is already maximum
	req = NewRequestf(t, "PUT", "/api/v1/repos/%s/%s/topics/%s?token=%s", user2.Name, repo2.Name, "t26", token2)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// Test delete a topic that repo doesn't have
	req = NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/topics/%s?token=%s", user2.Name, repo2.Name, "Topicname1", token2)
	MakeRequest(t, req, http.StatusNotFound)

	// Get user4's token
	token4 := getUserToken(t, user4.Name, auth_model.AccessTokenScopeWriteRepository)

	// Test read topics with write access
	url = fmt.Sprintf("/api/v1/repos/%s/%s/topics?token=%s", user3.Name, repo3.Name, token4)
	req = NewRequest(t, "GET", url)
	res = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.Empty(t, topics.TopicNames)

	// Test add a topic to repo with write access (requires repo admin access)
	req = NewRequestf(t, "PUT", "/api/v1/repos/%s/%s/topics/%s?token=%s", user3.Name, repo3.Name, "topicName", token4)
	MakeRequest(t, req, http.StatusForbidden)
}
