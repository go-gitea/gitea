// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// GitHook represents a Git repository hook
type GitHook struct {
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
	Content  string `json:"content,omitempty"`
}

// ListRepoGitHooksOptions options for listing repository's githooks
type ListRepoGitHooksOptions struct {
	ListOptions
}

// ListRepoGitHooks list all the Git hooks of one repository
func (c *Client) ListRepoGitHooks(user, repo string, opt ListRepoGitHooksOptions) ([]*GitHook, error) {
	opt.setDefaults()
	hooks := make([]*GitHook, 0, opt.PageSize)
	return hooks, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/hooks/git?%s", user, repo, opt.getURLQuery().Encode()), nil, nil, &hooks)
}

// GetRepoGitHook get a Git hook of a repository
func (c *Client) GetRepoGitHook(user, repo, id string) (*GitHook, error) {
	h := new(GitHook)
	return h, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/hooks/git/%s", user, repo, id), nil, nil, h)
}

// EditGitHookOption options when modifying one Git hook
type EditGitHookOption struct {
	Content string `json:"content"`
}

// EditRepoGitHook modify one Git hook of a repository
func (c *Client) EditRepoGitHook(user, repo, id string, opt EditGitHookOption) error {
	body, err := json.Marshal(&opt)
	if err != nil {
		return err
	}
	_, err = c.getResponse("PATCH", fmt.Sprintf("/repos/%s/%s/hooks/git/%s", user, repo, id), jsonHeader, bytes.NewReader(body))
	return err
}

// DeleteRepoGitHook delete one Git hook from a repository
func (c *Client) DeleteRepoGitHook(user, repo, id string) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/hooks/git/%s", user, repo, id), nil, nil)
	return err
}
