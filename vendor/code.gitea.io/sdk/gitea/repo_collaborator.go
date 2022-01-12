// Copyright 2021 The Gitea Authors. All rights reserved.
// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ListCollaboratorsOptions options for listing a repository's collaborators
type ListCollaboratorsOptions struct {
	ListOptions
}

// ListCollaborators list a repository's collaborators
func (c *Client) ListCollaborators(user, repo string, opt ListCollaboratorsOptions) ([]*User, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	collaborators := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/repos/%s/%s/collaborators?%s", user, repo, opt.getURLQuery().Encode()),
		nil, nil, &collaborators)
	return collaborators, resp, err
}

// IsCollaborator check if a user is a collaborator of a repository
func (c *Client) IsCollaborator(user, repo, collaborator string) (bool, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &collaborator); err != nil {
		return false, nil, err
	}
	status, resp, err := c.getStatusCode("GET", fmt.Sprintf("/repos/%s/%s/collaborators/%s", user, repo, collaborator), nil, nil)
	if err != nil {
		return false, resp, err
	}
	if status == 204 {
		return true, resp, nil
	}
	return false, resp, nil
}

// AddCollaboratorOption options when adding a user as a collaborator of a repository
type AddCollaboratorOption struct {
	Permission *AccessMode `json:"permission"`
}

// AccessMode represent the grade of access you have to something
type AccessMode string

const (
	// AccessModeNone no access
	AccessModeNone AccessMode = "none"
	// AccessModeRead read access
	AccessModeRead AccessMode = "read"
	// AccessModeWrite write access
	AccessModeWrite AccessMode = "write"
	// AccessModeAdmin admin access
	AccessModeAdmin AccessMode = "admin"
	// AccessModeOwner owner
	AccessModeOwner AccessMode = "owner"
)

// Validate the AddCollaboratorOption struct
func (opt AddCollaboratorOption) Validate() error {
	if opt.Permission != nil {
		if *opt.Permission == AccessModeOwner {
			*opt.Permission = AccessModeAdmin
			return nil
		}
		if *opt.Permission == AccessModeNone {
			opt.Permission = nil
			return nil
		}
		if *opt.Permission != AccessModeRead && *opt.Permission != AccessModeWrite && *opt.Permission != AccessModeAdmin {
			return fmt.Errorf("permission mode invalid")
		}
	}
	return nil
}

// AddCollaborator add some user as a collaborator of a repository
func (c *Client) AddCollaborator(user, repo, collaborator string, opt AddCollaboratorOption) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &collaborator); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("PUT", fmt.Sprintf("/repos/%s/%s/collaborators/%s", user, repo, collaborator), jsonHeader, bytes.NewReader(body))
	return resp, err
}

// DeleteCollaborator remove a collaborator from a repository
func (c *Client) DeleteCollaborator(user, repo, collaborator string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &collaborator); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE",
		fmt.Sprintf("/repos/%s/%s/collaborators/%s", user, repo, collaborator), nil, nil)
	return resp, err
}

// GetReviewers return all users that can be requested to review in this repo
func (c *Client) GetReviewers(user, repo string) ([]*User, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_15_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	reviewers := make([]*User, 0, 5)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/reviewers", user, repo), nil, nil, &reviewers)
	return reviewers, resp, err
}

// GetAssignees return all users that have write access and can be assigned to issues
func (c *Client) GetAssignees(user, repo string) ([]*User, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_15_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	assignees := make([]*User, 0, 5)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/assignees", user, repo), nil, nil, &assignees)
	return assignees, resp, err
}
