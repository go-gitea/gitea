// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogs

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func (c *Client) AdminCreateOrg(user string, opt CreateOrgOption) (*Organization, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	org := new(Organization)
	return org, c.getParsedResponse("POST", fmt.Sprintf("/admin/users/%s/orgs", user),
		jsonHeader, bytes.NewReader(body), org)
}

func (c *Client) AdminCreateTeam(user string, opt CreateTeamOption) (*Team, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	team := new(Team)
	return team, c.getParsedResponse("POST", fmt.Sprintf("/admin/orgs/%s/teams", user),
		jsonHeader, bytes.NewReader(body), team)
}

func (c *Client) AdminAddTeamMembership(teamID int64, user string) error {
	_, err := c.getResponse("PUT", fmt.Sprintf("/admin/teams/%d/members/%s", teamID, user),
		jsonHeader, nil)
	return err
}

func (c *Client) AdminAddTeamRepository(teamID int64, repo string) error {
	_, err := c.getResponse("PUT", fmt.Sprintf("/admin/teams/%d/repos/%s", teamID, repo),
		jsonHeader, nil)
	return err
}
