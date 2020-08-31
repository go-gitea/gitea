// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/http"
	"net/url"
)

// DeleteOrgMembership remove a member from an organization
func (c *Client) DeleteOrgMembership(org, user string) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/orgs/%s/members/%s", url.PathEscape(org), url.PathEscape(user)), nil, nil)
	return err
}

// ListOrgMembershipOption list OrgMembership options
type ListOrgMembershipOption struct {
	ListOptions
}

// ListOrgMembership list an organization's members
func (c *Client) ListOrgMembership(org string, opt ListOrgMembershipOption) ([]*User, error) {
	opt.setDefaults()
	users := make([]*User, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/members", url.PathEscape(org)))
	link.RawQuery = opt.getURLQuery().Encode()
	return users, c.getParsedResponse("GET", link.String(), jsonHeader, nil, &users)
}

// ListPublicOrgMembership list an organization's members
func (c *Client) ListPublicOrgMembership(org string, opt ListOrgMembershipOption) ([]*User, error) {
	opt.setDefaults()
	users := make([]*User, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/public_members", url.PathEscape(org)))
	link.RawQuery = opt.getURLQuery().Encode()
	return users, c.getParsedResponse("GET", link.String(), jsonHeader, nil, &users)
}

// CheckOrgMembership Check if a user is a member of an organization
func (c *Client) CheckOrgMembership(org, user string) (bool, error) {
	status, err := c.getStatusCode("GET", fmt.Sprintf("/orgs/%s/members/%s", url.PathEscape(org), url.PathEscape(user)), nil, nil)
	if err != nil {
		return false, err
	}
	switch status {
	case http.StatusNoContent:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected Status: %d", status)
	}
}

// CheckPublicOrgMembership Check if a user is a member of an organization
func (c *Client) CheckPublicOrgMembership(org, user string) (bool, error) {
	status, err := c.getStatusCode("GET", fmt.Sprintf("/orgs/%s/public_members/%s", url.PathEscape(org), url.PathEscape(user)), nil, nil)
	if err != nil {
		return false, err
	}
	switch status {
	case http.StatusNoContent:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected Status: %d", status)
	}
}

// SetPublicOrgMembership publicize/conceal a user's membership
func (c *Client) SetPublicOrgMembership(org, user string, visible bool) error {
	var (
		status int
		err    error
	)
	if visible {
		status, err = c.getStatusCode("PUT", fmt.Sprintf("/orgs/%s/public_members/%s", url.PathEscape(org), url.PathEscape(user)), nil, nil)
	} else {
		status, err = c.getStatusCode("DELETE", fmt.Sprintf("/orgs/%s/public_members/%s", url.PathEscape(org), url.PathEscape(user)), nil, nil)
	}
	if err != nil {
		return err
	}
	switch status {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("forbidden")
	default:
		return fmt.Errorf("unexpected Status: %d", status)
	}
}
