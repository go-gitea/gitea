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

// HookType represent all webhook types gitea currently offer
type HookType string

const (
	// HookTypeDingtalk webhook that dingtalk understand
	HookTypeDingtalk HookType = "dingtalk"
	// HookTypeDiscord webhook that discord understand
	HookTypeDiscord HookType = "discord"
	// HookTypeGitea webhook that gitea understand
	HookTypeGitea HookType = "gitea"
	// HookTypeGogs webhook that gogs understand
	HookTypeGogs HookType = "gogs"
	// HookTypeMsteams webhook that msteams understand
	HookTypeMsteams HookType = "msteams"
	// HookTypeSlack webhook that slack understand
	HookTypeSlack HookType = "slack"
	// HookTypeTelegram webhook that telegram understand
	HookTypeTelegram HookType = "telegram"
	// HookTypeFeishu webhook that feishu understand
	HookTypeFeishu HookType = "feishu"
)

// ListHooksOptions options for listing hooks
type ListHooksOptions struct {
	ListOptions
}

// ListOrgHooks list all the hooks of one organization
func (c *Client) ListOrgHooks(org string, opt ListHooksOptions) ([]*Hook, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	hooks := make([]*Hook, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/orgs/%s/hooks?%s", org, opt.getURLQuery().Encode()), nil, nil, &hooks)
	return hooks, resp, err
}

// ListRepoHooks list all the hooks of one repository
func (c *Client) ListRepoHooks(user, repo string, opt ListHooksOptions) ([]*Hook, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	hooks := make([]*Hook, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/hooks?%s", user, repo, opt.getURLQuery().Encode()), nil, nil, &hooks)
	return hooks, resp, err
}

// GetOrgHook get a hook of an organization
func (c *Client) GetOrgHook(org string, id int64) (*Hook, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	h := new(Hook)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/orgs/%s/hooks/%d", org, id), nil, nil, h)
	return h, resp, err
}

// GetRepoHook get a hook of a repository
func (c *Client) GetRepoHook(user, repo string, id int64) (*Hook, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	h := new(Hook)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/hooks/%d", user, repo, id), nil, nil, h)
	return h, resp, err
}

// CreateHookOption options when create a hook
type CreateHookOption struct {
	Type         HookType          `json:"type"`
	Config       map[string]string `json:"config"`
	Events       []string          `json:"events"`
	BranchFilter string            `json:"branch_filter"`
	Active       bool              `json:"active"`
}

// Validate the CreateHookOption struct
func (opt CreateHookOption) Validate() error {
	if len(opt.Type) == 0 {
		return fmt.Errorf("hook type needed")
	}
	return nil
}

// CreateOrgHook create one hook for an organization, with options
func (c *Client) CreateOrgHook(org string, opt CreateHookOption) (*Hook, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	h := new(Hook)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/orgs/%s/hooks", org), jsonHeader, bytes.NewReader(body), h)
	return h, resp, err
}

// CreateRepoHook create one hook for a repository, with options
func (c *Client) CreateRepoHook(user, repo string, opt CreateHookOption) (*Hook, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	h := new(Hook)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/hooks", user, repo), jsonHeader, bytes.NewReader(body), h)
	return h, resp, err
}

// EditHookOption options when modify one hook
type EditHookOption struct {
	Config       map[string]string `json:"config"`
	Events       []string          `json:"events"`
	BranchFilter string            `json:"branch_filter"`
	Active       *bool             `json:"active"`
}

// EditOrgHook modify one hook of an organization, with hook id and options
func (c *Client) EditOrgHook(org string, id int64, opt EditHookOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("PATCH", fmt.Sprintf("/orgs/%s/hooks/%d", org, id), jsonHeader, bytes.NewReader(body))
	return resp, err
}

// EditRepoHook modify one hook of a repository, with hook id and options
func (c *Client) EditRepoHook(user, repo string, id int64, opt EditHookOption) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("PATCH", fmt.Sprintf("/repos/%s/%s/hooks/%d", user, repo, id), jsonHeader, bytes.NewReader(body))
	return resp, err
}

// DeleteOrgHook delete one hook from an organization, with hook id
func (c *Client) DeleteOrgHook(org string, id int64) (*Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/orgs/%s/hooks/%d", org, id), nil, nil)
	return resp, err
}

// DeleteRepoHook delete one hook from a repository, with hook id
func (c *Client) DeleteRepoHook(user, repo string, id int64) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/hooks/%d", user, repo, id), nil, nil)
	return resp, err
}
