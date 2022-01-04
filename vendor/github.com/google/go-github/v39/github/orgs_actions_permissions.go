// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// ActionsPermissions represents a policy for repositories and allowed actions in an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/actions#permissions
type ActionsPermissions struct {
	EnabledRepositories *string `json:"enabled_repositories,omitempty"`
	AllowedActions      *string `json:"allowed_actions,omitempty"`
	SelectedActionsURL  *string `json:"selected_actions_url,omitempty"`
}

func (a ActionsPermissions) String() string {
	return Stringify(a)
}

// GetActionsPermissions gets the GitHub Actions permissions policy for repositories and allowed actions in an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/actions#get-github-actions-permissions-for-an-organization
func (s *OrganizationsService) GetActionsPermissions(ctx context.Context, org string) (*ActionsPermissions, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions", org)

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	permissions := new(ActionsPermissions)
	resp, err := s.client.Do(ctx, req, permissions)
	if err != nil {
		return nil, resp, err
	}

	return permissions, resp, nil
}

// EditActionsPermissions sets the permissions policy for repositories and allowed actions in an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/actions#set-github-actions-permissions-for-an-organization
func (s *OrganizationsService) EditActionsPermissions(ctx context.Context, org string, actionsPermissions ActionsPermissions) (*ActionsPermissions, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions", org)
	req, err := s.client.NewRequest("PUT", u, actionsPermissions)
	if err != nil {
		return nil, nil, err
	}
	p := new(ActionsPermissions)
	resp, err := s.client.Do(ctx, req, p)
	return p, resp, err
}
