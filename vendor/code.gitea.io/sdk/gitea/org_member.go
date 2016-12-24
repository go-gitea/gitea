// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// AddOrgMembershipOption add user to organization options
type AddOrgMembershipOption struct {
	Role string `json:"role" binding:"Required"`
}

// AddOrgMembership add some one to an organization's member
func (c *Client) AddOrgMembership(org, user string, opt AddOrgMembershipOption) error {
	body, err := json.Marshal(&opt)
	if err != nil {
		return err
	}
	_, err = c.getResponse("PUT", fmt.Sprintf("/orgs/%s/membership/%s", org, user), jsonHeader, bytes.NewReader(body))
	return err
}
