// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogs

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Collaborator struct {
	*User
	Permissions Permission `json:"permissions"`
}

type AddCollaboratorOption struct {
	Permission *string `json:"permission"`
}

func (c *Client) ListCollaborator(user, repo string) ([]*Collaborator, error) {
	collabs := make([]*Collaborator, 0, 10)
	return collabs, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/collaborators", user, repo), nil, nil, &collabs)
}

func (c *Client) AddCollaborator(user, repo, collaborator string, opt AddCollaboratorOption) error {
	body, err := json.Marshal(&opt)
	if err != nil {
		return err
	}
	_, err = c.getResponse("PUT", fmt.Sprintf("/repos/%s/%s/collaborators/%s", user, repo, collaborator), jsonHeader, bytes.NewReader(body))
	return err
}

func (c *Client) DeleteCollaborator(user, repo, collaborator string) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/collaborators/%s", user, repo, collaborator), nil, nil)
	return err
}

func (c *Client) IsCollaborator(user, repo, collaborator string) error {
	_, err := c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/collaborators/%s", user, repo, collaborator), nil, nil)
	return err
}
