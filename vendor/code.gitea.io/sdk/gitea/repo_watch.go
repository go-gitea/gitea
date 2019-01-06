// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/http"
	"time"
)

// WatchInfo represents an API watch status of one repository
type WatchInfo struct {
	Subscribed    bool        `json:"subscribed"`
	Ignored       bool        `json:"ignored"`
	Reason        interface{} `json:"reason"`
	CreatedAt     time.Time   `json:"created_at"`
	URL           string      `json:"url"`
	RepositoryURL string      `json:"repository_url"`
}

// GetWatchedRepos list all the watched repos of user
func (c *Client) GetWatchedRepos(user, pass string) ([]*Repository, error) {
	repos := make([]*Repository, 0, 10)
	return repos, c.getParsedResponse("GET", fmt.Sprintf("/users/%s/subscriptions", user),
		http.Header{"Authorization": []string{"Basic " + BasicAuthEncode(user, pass)}}, nil, &repos)
}

// WatchRepo start to watch a repository
func (c *Client) WatchRepo(user, pass, repoUser, repoName string) (*WatchInfo, error) {
	i := new(WatchInfo)
	return i, c.getParsedResponse("PUT", fmt.Sprintf("/repos/%s/%s/subscription", repoUser, repoName),
		http.Header{"Authorization": []string{"Basic " + BasicAuthEncode(user, pass)}}, nil, i)
}

// UnWatchRepo start to watch a repository
func (c *Client) UnWatchRepo(user, pass, repoUser, repoName string) (int, error) {
	return c.getStatusCode("DELETE", fmt.Sprintf("/repos/%s/%s/subscription", repoUser, repoName),
		http.Header{"Authorization": []string{"Basic " + BasicAuthEncode(user, pass)}}, nil)
}
