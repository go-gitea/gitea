// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/http"
)

// GetRepoTeams return teams from a repository
func (c *Client) GetRepoTeams(user, repo string) ([]*Team, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_15_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	teams := make([]*Team, 0, 5)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/teams", user, repo), nil, nil, &teams)
	return teams, resp, err
}

// AddRepoTeam add a team to a repository
func (c *Client) AddRepoTeam(user, repo, team string) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_15_0); err != nil {
		return nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo, &team); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("PUT", fmt.Sprintf("/repos/%s/%s/teams/%s", user, repo, team), nil, nil)
	return resp, err
}

// RemoveRepoTeam delete a team from a repository
func (c *Client) RemoveRepoTeam(user, repo, team string) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_15_0); err != nil {
		return nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo, &team); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/teams/%s", user, repo, team), nil, nil)
	return resp, err
}

// CheckRepoTeam check if team is assigned to repo by name and return it.
// If not assigned, it will return nil.
func (c *Client) CheckRepoTeam(user, repo, team string) (*Team, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_15_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo, &team); err != nil {
		return nil, nil, err
	}
	t := new(Team)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/teams/%s", user, repo, team), nil, nil, &t)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		// if not found it's not an error, it indicates it's not assigned
		return nil, resp, nil
	}
	return t, resp, err
}
