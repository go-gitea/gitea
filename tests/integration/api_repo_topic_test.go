// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIRepoTopic(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("Search", func(t *testing.T) {
		searchURL, _ := url.Parse("/api/v1/topics/search")

		for _, testCase := range []struct {
			query url.Values
			count int
			total string
		}{
			// search all topics
			{nil, 6, "6"},
			// pagination search topics first page
			{url.Values{"page": {"1"}, "limit": {"4"}}, 4, "6"},
			// pagination search topics second page
			{url.Values{"page": {"2"}, "limit": {"4"}}, 2, "6"},
			// add keyword search
			{url.Values{"page": {"1"}, "limit": {"4"}, "q": {"topic"}}, 2, ""},
			{url.Values{"page": {"1"}, "limit": {"4"}, "q": {"database"}}, 1, ""},
		} {
			searchURL.RawQuery = testCase.query.Encode()
			res := MakeRequest(t, NewRequest(t, "GET", searchURL.String()), http.StatusOK)
			topics := DecodeJSON(t, res, &struct {
				TopicNames []*api.TopicResponse `json:"topics"`
			}{})
			validLength := assert.Len(t, topics.TopicNames, testCase.count)
			if testCase.total != "" {
				assert.Equal(t, testCase.total, res.Header().Get("x-total-count"))
			}
			if testCase.query.Get("q") == "database" && validLength {
				assert.EqualValues(t, 2, topics.TopicNames[0].ID)
				assert.Equal(t, "database", topics.TopicNames[0].Name)
				assert.Equal(t, 1, topics.TopicNames[0].RepoCount)
			}
		}
	})

	t.Run("Archived", func(t *testing.T) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 30}) // owner of the archived repo51
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 51})
		assert.True(t, repo.IsArchived)
		token := getUserToken(t, user.Name, auth_model.AccessTokenScopeWriteRepository)

		// writing topics on an archived repo must be rejected, matching the web UI
		req := NewRequestf(t, "PUT", "/api/v1/repos/%s/%s/topics/%s", user.Name, repo.Name, "archivedtopic").
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusLocked)

		req = NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/topics/%s", user.Name, repo.Name, "archivedtopic").
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusLocked)

		req = NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/topics", user.Name, repo.Name),
			&api.RepoTopicOptions{Topics: []string{"archivedtopic"}}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusLocked)

		// reading topics stays allowed on an archived repo
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/topics", user.Name, repo.Name)).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)
	})

	t.Run("Repository", func(t *testing.T) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}) // owner of repo2
		org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})  // owner of repo3
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}) // write access to repo 3
		repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
		repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})

		// Get user2's token
		token2 := getUserToken(t, user2.Name, auth_model.AccessTokenScopeWriteRepository)

		// Test read topics using login
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/topics", user2.Name, repo2.Name)).
			AddTokenAuth(token2)
		res := MakeRequest(t, req, http.StatusOK)
		topics := DecodeJSON(t, res, &api.TopicName{})
		assert.ElementsMatch(t, []string{"topicname1", "topicname2"}, topics.TopicNames)

		// Test delete a topic
		req = NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/topics/%s", user2.Name, repo2.Name, "Topicname1").
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusNoContent)

		// Test add an existing topic
		req = NewRequestf(t, "PUT", "/api/v1/repos/%s/%s/topics/%s", user2.Name, repo2.Name, "Golang").
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusNoContent)

		// Test add a topic
		req = NewRequestf(t, "PUT", "/api/v1/repos/%s/%s/topics/%s", user2.Name, repo2.Name, "topicName3").
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusNoContent)

		url := fmt.Sprintf("/api/v1/repos/%s/%s/topics", user2.Name, repo2.Name)

		// Test read topics using token
		req = NewRequest(t, "GET", url).
			AddTokenAuth(token2)
		res = MakeRequest(t, req, http.StatusOK)
		topics = DecodeJSON(t, res, &api.TopicName{})
		assert.ElementsMatch(t, []string{"topicname2", "golang", "topicname3"}, topics.TopicNames)

		newTopics := []string{"t1", "t2", "t1", "t3", "t4", "t5", "t6", "t7", "t8", "t9", "t10", "t11", "t12", "t13", "t14", "t15", "t16", "17", "t18", "t19", "t20", "t21", "t22", "t23", "t24", "t25"}
		for _, testCase := range []struct {
			topics   []string
			status   int
			expected []string
		}{
			// Test replace topics
			{[]string{"   windows ", "   ", "MAC  "}, http.StatusNoContent, []string{"windows", "mac"}},
			// Test replace topics with something invalid
			{[]string{"topicname1", "topicname2", "topicname!"}, http.StatusUnprocessableEntity, []string{"windows", "mac"}},
			// Test with some topics multiple times, less than 25 unique
			{newTopics, http.StatusNoContent, nil},
		} {
			req = NewRequestWithJSON(t, "PUT", url, &api.RepoTopicOptions{
				Topics: testCase.topics,
			}).AddTokenAuth(token2)
			MakeRequest(t, req, testCase.status)
			req = NewRequest(t, "GET", url).
				AddTokenAuth(token2)
			res = MakeRequest(t, req, http.StatusOK)
			topics = DecodeJSON(t, res, &api.TopicName{})
			if testCase.expected != nil {
				assert.ElementsMatch(t, testCase.expected, topics.TopicNames)
			} else {
				assert.Len(t, topics.TopicNames, 25)
			}
		}

		// Test writing more topics than allowed
		newTopics = append(newTopics, "t26")
		req = NewRequestWithJSON(t, "PUT", url, &api.RepoTopicOptions{
			Topics: newTopics,
		}).AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusUnprocessableEntity)

		// Test add a topic when there is already maximum
		req = NewRequestf(t, "PUT", "/api/v1/repos/%s/%s/topics/%s", user2.Name, repo2.Name, "t26").
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusUnprocessableEntity)

		// Test delete a topic that repo doesn't have
		req = NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/topics/%s", user2.Name, repo2.Name, "Topicname1").
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusNotFound)

		// Get user4's token
		token4 := getUserToken(t, user4.Name, auth_model.AccessTokenScopeWriteRepository)

		// Test read topics with write access
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/topics", org3.Name, repo3.Name)).
			AddTokenAuth(token4)
		res = MakeRequest(t, req, http.StatusOK)
		topics = DecodeJSON(t, res, &api.TopicName{})
		assert.Empty(t, topics.TopicNames)

		// Test add a topic to repo with write access (requires repo admin access)
		req = NewRequestf(t, "PUT", "/api/v1/repos/%s/%s/topics/%s", org3.Name, repo3.Name, "topicName").
			AddTokenAuth(token4)
		MakeRequest(t, req, http.StatusForbidden)
	})
}
