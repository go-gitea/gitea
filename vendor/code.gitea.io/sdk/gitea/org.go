// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Organization represents an organization
type Organization struct {
	ID          int64  `json:"id"`
	UserName    string `json:"username"`
	FullName    string `json:"full_name"`
	AvatarURL   string `json:"avatar_url"`
	Description string `json:"description"`
	Website     string `json:"website"`
	Location    string `json:"location"`
	Visibility  string `json:"visibility"`
}

// VisibleType defines the visibility
type VisibleType string

const (
	// VisibleTypePublic Visible for everyone
	VisibleTypePublic VisibleType = "public"

	// VisibleTypeLimited Visible for every connected user
	VisibleTypeLimited VisibleType = "limited"

	// VisibleTypePrivate Visible only for organization's members
	VisibleTypePrivate VisibleType = "private"
)

// ListOrgsOptions options for listing organizations
type ListOrgsOptions struct {
	ListOptions
}

// ListMyOrgs list all of current user's organizations
func (c *Client) ListMyOrgs(opt ListOrgsOptions) ([]*Organization, *Response, error) {
	opt.setDefaults()
	orgs := make([]*Organization, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/user/orgs?%s", opt.getURLQuery().Encode()), nil, nil, &orgs)
	return orgs, resp, err
}

// ListUserOrgs list all of some user's organizations
func (c *Client) ListUserOrgs(user string, opt ListOrgsOptions) ([]*Organization, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	orgs := make([]*Organization, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/users/%s/orgs?%s", user, opt.getURLQuery().Encode()), nil, nil, &orgs)
	return orgs, resp, err
}

// GetOrg get one organization by name
func (c *Client) GetOrg(orgname string) (*Organization, *Response, error) {
	if err := escapeValidatePathSegments(&orgname); err != nil {
		return nil, nil, err
	}
	org := new(Organization)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/orgs/%s", orgname), nil, nil, org)
	return org, resp, err
}

// CreateOrgOption options for creating an organization
type CreateOrgOption struct {
	Name                      string      `json:"username"`
	FullName                  string      `json:"full_name"`
	Description               string      `json:"description"`
	Website                   string      `json:"website"`
	Location                  string      `json:"location"`
	Visibility                VisibleType `json:"visibility"`
	RepoAdminChangeTeamAccess bool        `json:"repo_admin_change_team_access"`
}

// checkVisibilityOpt check if mode exist
func checkVisibilityOpt(v VisibleType) bool {
	return v == VisibleTypePublic || v == VisibleTypeLimited || v == VisibleTypePrivate
}

// Validate the CreateOrgOption struct
func (opt CreateOrgOption) Validate() error {
	if len(opt.Name) == 0 {
		return fmt.Errorf("empty org name")
	}
	if len(opt.Visibility) != 0 && !checkVisibilityOpt(opt.Visibility) {
		return fmt.Errorf("infalid bisibility option")
	}
	return nil
}

// CreateOrg creates an organization
func (c *Client) CreateOrg(opt CreateOrgOption) (*Organization, *Response, error) {
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	org := new(Organization)
	resp, err := c.getParsedResponse("POST", "/orgs", jsonHeader, bytes.NewReader(body), org)
	return org, resp, err
}

// EditOrgOption options for editing an organization
type EditOrgOption struct {
	FullName    string      `json:"full_name"`
	Description string      `json:"description"`
	Website     string      `json:"website"`
	Location    string      `json:"location"`
	Visibility  VisibleType `json:"visibility"`
}

// Validate the EditOrgOption struct
func (opt EditOrgOption) Validate() error {
	if len(opt.Visibility) != 0 && !checkVisibilityOpt(opt.Visibility) {
		return fmt.Errorf("infalid bisibility option")
	}
	return nil
}

// EditOrg modify one organization via options
func (c *Client) EditOrg(orgname string, opt EditOrgOption) (*Response, error) {
	if err := escapeValidatePathSegments(&orgname); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("PATCH", fmt.Sprintf("/orgs/%s", orgname), jsonHeader, bytes.NewReader(body))
	return resp, err
}

// DeleteOrg deletes an organization
func (c *Client) DeleteOrg(orgname string) (*Response, error) {
	if err := escapeValidatePathSegments(&orgname); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/orgs/%s", orgname), jsonHeader, nil)
	return resp, err
}
