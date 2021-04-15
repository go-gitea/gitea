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

// AdminListOrgsOptions options for listing admin's organizations
type AdminListOrgsOptions struct {
	ListOptions
}

// AdminListOrgs lists all orgs
func (c *Client) AdminListOrgs(opt AdminListOrgsOptions) ([]*Organization, *Response, error) {
	opt.setDefaults()
	orgs := make([]*Organization, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/admin/orgs?%s", opt.getURLQuery().Encode()), nil, nil, &orgs)
	return orgs, resp, err
}

// AdminCreateOrg create an organization
func (c *Client) AdminCreateOrg(user string, opt CreateOrgOption) (*Organization, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	org := new(Organization)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/admin/users/%s/orgs", user), jsonHeader, bytes.NewReader(body), org)
	return org, resp, err
}
