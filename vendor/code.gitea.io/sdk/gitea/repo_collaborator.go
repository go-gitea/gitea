// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ListCollaborators list a repository's collaborators
func (c *Client) ListCollaborators(user, repo string) ([]*User, error) {
	collaborators := make([]*User, 0, 10)
	err := c.getParsedResponse("GET",
		fmt.Sprintf("/repos/%s/%s/collaborators", user, repo),
		nil, nil, &collaborators)
	return collaborators, err
}

// IsCollaborator check if a user is a collaborator of a repository
func (c *Client) IsCollaborator(user, repo, collaborator string) (bool, error) {
	status, err := c.getStatusCode("GET",
		fmt.Sprintf("/repos/%s/%s/collaborators/%s", user, repo, collaborator),
		nil, nil)
	if err != nil {
		return false, err
	}
	if status == 204 {
		return true, nil
	}
	return false, nil
}

// AddCollaboratorOption options when adding a user as a collaborator of a repository
type AddCollaboratorOption struct {
	Permission *string `json:"permission"`
}

// AddCollaborator add some user as a collaborator of a repository
func (c *Client) AddCollaborator(user, repo, collaborator string, opt AddCollaboratorOption) error {
	body, err := json.Marshal(&opt)
	if err != nil {
		return err
	}
	_, err = c.getResponse("PUT", fmt.Sprintf("/repos/%s/%s/collaborators/%s", user, repo, collaborator), nil, bytes.NewReader(body))
	return err
}

// DeleteCollaborator remove a collaborator from a repository
func (c *Client) DeleteCollaborator(user, repo, collaborator string) error {
	_, err := c.getResponse("DELETE",
		fmt.Sprintf("/repos/%s/%s/collaborators/%s", user, repo, collaborator),
		nil, nil)
	return err
}
