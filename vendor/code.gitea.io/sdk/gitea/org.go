// Copyright 2015 The Gogs Authors. All rights reserved.
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
}

// ListMyOrgs list all of current user's organizations
func (c *Client) ListMyOrgs() ([]*Organization, error) {
	orgs := make([]*Organization, 0, 5)
	return orgs, c.getParsedResponse("GET", "/user/orgs", nil, nil, &orgs)
}

// ListUserOrgs list all of some user's organizations
func (c *Client) ListUserOrgs(user string) ([]*Organization, error) {
	orgs := make([]*Organization, 0, 5)
	return orgs, c.getParsedResponse("GET", fmt.Sprintf("/users/%s/orgs", user), nil, nil, &orgs)
}

// GetOrg get one organization by name
func (c *Client) GetOrg(orgname string) (*Organization, error) {
	org := new(Organization)
	return org, c.getParsedResponse("GET", fmt.Sprintf("/orgs/%s", orgname), nil, nil, org)
}

// CreateOrgOption options for creating an organization
type CreateOrgOption struct {
	// required: true
	UserName string `json:"username" binding:"Required"`
	FullName string `json:"full_name"`
	Description string `json:"description"`
	Website string `json:"website"`
	Location string `json:"location"`
}

// EditOrgOption options for editing an organization
type EditOrgOption struct {
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Website     string `json:"website"`
	Location    string `json:"location"`
}

// EditOrg modify one organization via options
func (c *Client) EditOrg(orgname string, opt EditOrgOption) error {
	body, err := json.Marshal(&opt)
	if err != nil {
		return err
	}
	_, err = c.getResponse("PATCH", fmt.Sprintf("/orgs/%s", orgname), jsonHeader, bytes.NewReader(body))
	return err
}
