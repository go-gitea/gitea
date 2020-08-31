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
func (c *Client) GetWatchedRepos(user string) ([]*Repository, error) {
	repos := make([]*Repository, 0, 10)
	return repos, c.getParsedResponse("GET", fmt.Sprintf("/users/%s/subscriptions", user), nil, nil, &repos)
}

// GetMyWatchedRepos list repositories watched by the authenticated user
func (c *Client) GetMyWatchedRepos() ([]*Repository, error) {
	repos := make([]*Repository, 0, 10)
	return repos, c.getParsedResponse("GET", fmt.Sprintf("/user/subscriptions"), nil, nil, &repos)
}

// CheckRepoWatch check if the current user is watching a repo
func (c *Client) CheckRepoWatch(repoUser, repoName string) (bool, error) {
	status, err := c.getStatusCode("GET", fmt.Sprintf("/repos/%s/%s/subscription", repoUser, repoName), nil, nil)
	if err != nil {
		return false, err
	}
	switch status {
	case http.StatusNotFound:
		return false, nil
	case http.StatusOK:
		return true, nil
	default:
		return false, fmt.Errorf("unexpected Status: %d", status)
	}
}

// WatchRepo start to watch a repository
func (c *Client) WatchRepo(repoUser, repoName string) error {
	status, err := c.getStatusCode("PUT", fmt.Sprintf("/repos/%s/%s/subscription", repoUser, repoName), nil, nil)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		return nil
	}
	return fmt.Errorf("unexpected Status: %d", status)
}

// UnWatchRepo stop to watch a repository
func (c *Client) UnWatchRepo(repoUser, repoName string) error {
	status, err := c.getStatusCode("DELETE", fmt.Sprintf("/repos/%s/%s/subscription", repoUser, repoName), nil, nil)
	if err != nil {
		return err
	}
	if status == http.StatusNoContent {
		return nil
	}
	return fmt.Errorf("unexpected Status: %d", status)
}
