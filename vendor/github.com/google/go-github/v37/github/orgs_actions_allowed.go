// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// ActionsAllowed represents selected actions that are allowed in an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/actions#get-allowed-actions-for-an-organization
type ActionsAllowed struct {
	GithubOwnedAllowed *bool    `json:"github_owned_allowed,omitempty"`
	VerifiedAllowed    *bool    `json:"verified_allowed,omitempty"`
	PatternsAllowed    []string `json:"patterns_allowed,omitempty"`
}

func (a ActionsAllowed) String() string {
	return Stringify(a)
}

// GetActionsAllowed gets the actions that are allowed in an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/actions#get-allowed-actions-for-an-organization
func (s *OrganizationsService) GetActionsAllowed(ctx context.Context, org string) (*ActionsAllowed, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/selected-actions", org)

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	actionsAllowed := new(ActionsAllowed)
	resp, err := s.client.Do(ctx, req, actionsAllowed)
	if err != nil {
		return nil, resp, err
	}

	return actionsAllowed, resp, nil
}

// EditActionsAllowed sets the actions that are allowed in an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/actions#set-allowed-actions-for-an-organization
func (s *OrganizationsService) EditActionsAllowed(ctx context.Context, org string, actionsAllowed ActionsAllowed) (*ActionsAllowed, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/selected-actions", org)
	req, err := s.client.NewRequest("PUT", u, actionsAllowed)
	if err != nil {
		return nil, nil, err
	}
	p := new(ActionsAllowed)
	resp, err := s.client.Do(ctx, req, p)
	return p, resp, err
}
