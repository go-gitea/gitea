// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Hook a hook is a web hook when one repository changed
type Hook struct {
	ID      int64             `json:"id"`
	Type    string            `json:"type"`
	URL     string            `json:"-"`
	Config  map[string]string `json:"config"`
	Events  []string          `json:"events"`
	Active  bool              `json:"active"`
	Updated time.Time         `json:"updated_at"`
	Created time.Time         `json:"created_at"`
}

// ListHooksOptions options for listing hooks
type ListHooksOptions struct {
	ListOptions
}

// ListOrgHooks list all the hooks of one organization
func (c *Client) ListOrgHooks(org string, opt ListHooksOptions) ([]*Hook, error) {
	opt.setDefaults()
	hooks := make([]*Hook, 0, opt.PageSize)
	return hooks, c.getParsedResponse("GET", fmt.Sprintf("/orgs/%s/hooks?%s", org, opt.getURLQuery().Encode()), nil, nil, &hooks)
}

// ListRepoHooks list all the hooks of one repository
func (c *Client) ListRepoHooks(user, repo string, opt ListHooksOptions) ([]*Hook, error) {
	opt.setDefaults()
	hooks := make([]*Hook, 0, opt.PageSize)
	return hooks, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/hooks?%s", user, repo, opt.getURLQuery().Encode()), nil, nil, &hooks)
}

// GetOrgHook get a hook of an organization
func (c *Client) GetOrgHook(org string, id int64) (*Hook, error) {
	h := new(Hook)
	return h, c.getParsedResponse("GET", fmt.Sprintf("/orgs/%s/hooks/%d", org, id), nil, nil, h)
}

// GetRepoHook get a hook of a repository
func (c *Client) GetRepoHook(user, repo string, id int64) (*Hook, error) {
	h := new(Hook)
	return h, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/hooks/%d", user, repo, id), nil, nil, h)
}

// CreateHookOption options when create a hook
type CreateHookOption struct {
	Type         string            `json:"type"`
	Config       map[string]string `json:"config"`
	Events       []string          `json:"events"`
	BranchFilter string            `json:"branch_filter"`
	Active       bool              `json:"active"`
}

// CreateOrgHook create one hook for an organization, with options
func (c *Client) CreateOrgHook(org string, opt CreateHookOption) (*Hook, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	h := new(Hook)
	return h, c.getParsedResponse("POST", fmt.Sprintf("/orgs/%s/hooks", org), jsonHeader, bytes.NewReader(body), h)
}

// CreateRepoHook create one hook for a repository, with options
func (c *Client) CreateRepoHook(user, repo string, opt CreateHookOption) (*Hook, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	h := new(Hook)
	return h, c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/hooks", user, repo), jsonHeader, bytes.NewReader(body), h)
}

// EditHookOption options when modify one hook
type EditHookOption struct {
	Config       map[string]string `json:"config"`
	Events       []string          `json:"events"`
	BranchFilter string            `json:"branch_filter"`
	Active       *bool             `json:"active"`
}

// EditOrgHook modify one hook of an organization, with hook id and options
func (c *Client) EditOrgHook(org string, id int64, opt EditHookOption) error {
	body, err := json.Marshal(&opt)
	if err != nil {
		return err
	}
	_, err = c.getResponse("PATCH", fmt.Sprintf("/orgs/%s/hooks/%d", org, id), jsonHeader, bytes.NewReader(body))
	return err
}

// EditRepoHook modify one hook of a repository, with hook id and options
func (c *Client) EditRepoHook(user, repo string, id int64, opt EditHookOption) error {
	body, err := json.Marshal(&opt)
	if err != nil {
		return err
	}
	_, err = c.getResponse("PATCH", fmt.Sprintf("/repos/%s/%s/hooks/%d", user, repo, id), jsonHeader, bytes.NewReader(body))
	return err
}

// DeleteOrgHook delete one hook from an organization, with hook id
func (c *Client) DeleteOrgHook(org string, id int64) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/orgs/%s/hooks/%d", org, id), nil, nil)
	return err
}

// DeleteRepoHook delete one hook from a repository, with hook id
func (c *Client) DeleteRepoHook(user, repo string, id int64) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/hooks/%d", user, repo, id), nil, nil)
	return err
}
