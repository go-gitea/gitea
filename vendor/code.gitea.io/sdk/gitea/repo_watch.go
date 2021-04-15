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
func (c *Client) GetWatchedRepos(user string) ([]*Repository, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	repos := make([]*Repository, 0, 10)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/users/%s/subscriptions", user), nil, nil, &repos)
	return repos, resp, err
}

// GetMyWatchedRepos list repositories watched by the authenticated user
func (c *Client) GetMyWatchedRepos() ([]*Repository, *Response, error) {
	repos := make([]*Repository, 0, 10)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/user/subscriptions"), nil, nil, &repos)
	return repos, resp, err
}

// CheckRepoWatch check if the current user is watching a repo
func (c *Client) CheckRepoWatch(owner, repo string) (bool, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return false, nil, err
	}
	status, resp, err := c.getStatusCode("GET", fmt.Sprintf("/repos/%s/%s/subscription", owner, repo), nil, nil)
	if err != nil {
		return false, resp, err
	}
	switch status {
	case http.StatusNotFound:
		return false, resp, nil
	case http.StatusOK:
		return true, resp, nil
	default:
		return false, resp, fmt.Errorf("unexpected Status: %d", status)
	}
}

// WatchRepo start to watch a repository
func (c *Client) WatchRepo(owner, repo string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("PUT", fmt.Sprintf("/repos/%s/%s/subscription", owner, repo), nil, nil)
	if err != nil {
		return resp, err
	}
	if status == http.StatusOK {
		return resp, nil
	}
	return resp, fmt.Errorf("unexpected Status: %d", status)
}

// UnWatchRepo stop to watch a repository
func (c *Client) UnWatchRepo(owner, repo string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("DELETE", fmt.Sprintf("/repos/%s/%s/subscription", owner, repo), nil, nil)
	if err != nil {
		return resp, err
	}
	if status == http.StatusNoContent {
		return resp, nil
	}
	return resp, fmt.Errorf("unexpected Status: %d", status)
}
