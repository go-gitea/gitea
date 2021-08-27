// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/http"
)

// ListStargazersOptions options for listing a repository's stargazers
type ListStargazersOptions struct {
	ListOptions
}

// ListRepoStargazers list a repository's stargazers
func (c *Client) ListRepoStargazers(user, repo string, opt ListStargazersOptions) ([]*User, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	stargazers := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/stargazers?%s", user, repo, opt.getURLQuery().Encode()), nil, nil, &stargazers)
	return stargazers, resp, err
}

// GetStarredRepos returns the repos that the given user has starred
func (c *Client) GetStarredRepos(user string) ([]*Repository, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	repos := make([]*Repository, 0, 10)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/users/%s/starred", user), jsonHeader, nil, &repos)
	return repos, resp, err
}

// GetMyStarredRepos returns the repos that the authenticated user has starred
func (c *Client) GetMyStarredRepos() ([]*Repository, *Response, error) {
	repos := make([]*Repository, 0, 10)
	resp, err := c.getParsedResponse("GET", "/user/starred", jsonHeader, nil, &repos)
	return repos, resp, err
}

// IsRepoStarring returns whether the authenticated user has starred the repo or not
func (c *Client) IsRepoStarring(user, repo string) (bool, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return false, nil, err
	}
	_, resp, err := c.getResponse("GET", fmt.Sprintf("/user/starred/%s/%s", user, repo), jsonHeader, nil)
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusNotFound:
			return false, resp, nil
		case http.StatusNoContent:
			return true, resp, nil
		default:
			return false, resp, fmt.Errorf("unexpected status code '%d'", resp.StatusCode)
		}
	}
	return false, nil, err
}

// StarRepo star specified repo as the authenticated user
func (c *Client) StarRepo(user, repo string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("PUT", fmt.Sprintf("/user/starred/%s/%s", user, repo), jsonHeader, nil)
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusNoContent:
			return resp, nil
		default:
			return resp, fmt.Errorf("unexpected status code '%d'", resp.StatusCode)
		}
	}
	return nil, err
}

// UnStarRepo remove star to specified repo as the authenticated user
func (c *Client) UnStarRepo(user, repo string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/user/starred/%s/%s", user, repo), jsonHeader, nil)
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusNoContent:
			return resp, nil
		default:
			return resp, fmt.Errorf("unexpected status code '%d'", resp.StatusCode)
		}
	}
	return nil, err
}
