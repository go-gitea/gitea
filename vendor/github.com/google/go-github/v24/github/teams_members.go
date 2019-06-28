// Copyright 2018 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// TeamListTeamMembersOptions specifies the optional parameters to the
// TeamsService.ListTeamMembers method.
type TeamListTeamMembersOptions struct {
	// Role filters members returned by their role in the team. Possible
	// values are "all", "member", "maintainer". Default is "all".
	Role string `url:"role,omitempty"`

	ListOptions
}

// ListTeamMembers lists all of the users who are members of the specified
// team.
//
// GitHub API docs: https://developer.github.com/v3/teams/members/#list-team-members
func (s *TeamsService) ListTeamMembers(ctx context.Context, team int64, opt *TeamListTeamMembersOptions) ([]*User, *Response, error) {
	u := fmt.Sprintf("teams/%v/members", team)
	u, err := addOptions(u, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Accept", mediaTypeNestedTeamsPreview)

	var members []*User
	resp, err := s.client.Do(ctx, req, &members)
	if err != nil {
		return nil, resp, err
	}

	return members, resp, nil
}

// IsTeamMember checks if a user is a member of the specified team.
//
// GitHub API docs: https://developer.github.com/v3/teams/members/#get-team-member
//
// Deprecated: This API has been marked as deprecated in the Github API docs,
// TeamsService.GetTeamMembership method should be used instead.
func (s *TeamsService) IsTeamMember(ctx context.Context, team int64, user string) (bool, *Response, error) {
	u := fmt.Sprintf("teams/%v/members/%v", team, user)
	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return false, nil, err
	}

	resp, err := s.client.Do(ctx, req, nil)
	member, err := parseBoolResponse(err)
	return member, resp, err
}

// GetTeamMembership returns the membership status for a user in a team.
//
// GitHub API docs: https://developer.github.com/v3/teams/members/#get-team-membership
func (s *TeamsService) GetTeamMembership(ctx context.Context, team int64, user string) (*Membership, *Response, error) {
	u := fmt.Sprintf("teams/%v/memberships/%v", team, user)
	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Accept", mediaTypeNestedTeamsPreview)

	t := new(Membership)
	resp, err := s.client.Do(ctx, req, t)
	if err != nil {
		return nil, resp, err
	}

	return t, resp, nil
}

// TeamAddTeamMembershipOptions specifies the optional
// parameters to the TeamsService.AddTeamMembership method.
type TeamAddTeamMembershipOptions struct {
	// Role specifies the role the user should have in the team. Possible
	// values are:
	//     member - a normal member of the team
	//     maintainer - a team maintainer. Able to add/remove other team
	//                  members, promote other team members to team
	//                  maintainer, and edit the team’s name and description
	//
	// Default value is "member".
	Role string `json:"role,omitempty"`
}

// AddTeamMembership adds or invites a user to a team.
//
// In order to add a membership between a user and a team, the authenticated
// user must have 'admin' permissions to the team or be an owner of the
// organization that the team is associated with.
//
// If the user is already a part of the team's organization (meaning they're on
// at least one other team in the organization), this endpoint will add the
// user to the team.
//
// If the user is completely unaffiliated with the team's organization (meaning
// they're on none of the organization's teams), this endpoint will send an
// invitation to the user via email. This newly-created membership will be in
// the "pending" state until the user accepts the invitation, at which point
// the membership will transition to the "active" state and the user will be
// added as a member of the team.
//
// GitHub API docs: https://developer.github.com/v3/teams/members/#add-or-update-team-membership
func (s *TeamsService) AddTeamMembership(ctx context.Context, team int64, user string, opt *TeamAddTeamMembershipOptions) (*Membership, *Response, error) {
	u := fmt.Sprintf("teams/%v/memberships/%v", team, user)
	req, err := s.client.NewRequest("PUT", u, opt)
	if err != nil {
		return nil, nil, err
	}

	t := new(Membership)
	resp, err := s.client.Do(ctx, req, t)
	if err != nil {
		return nil, resp, err
	}

	return t, resp, nil
}

// RemoveTeamMembership removes a user from a team.
//
// GitHub API docs: https://developer.github.com/v3/teams/members/#remove-team-membership
func (s *TeamsService) RemoveTeamMembership(ctx context.Context, team int64, user string) (*Response, error) {
	u := fmt.Sprintf("teams/%v/memberships/%v", team, user)
	req, err := s.client.NewRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(ctx, req, nil)
}

// ListPendingTeamInvitations get pending invitaion list in team.
// Warning: The API may change without advance notice during the preview period.
// Preview features are not supported for production use.
//
// GitHub API docs: https://developer.github.com/v3/teams/members/#list-pending-team-invitations
func (s *TeamsService) ListPendingTeamInvitations(ctx context.Context, team int64, opt *ListOptions) ([]*Invitation, *Response, error) {
	u := fmt.Sprintf("teams/%v/invitations", team)
	u, err := addOptions(u, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var pendingInvitations []*Invitation
	resp, err := s.client.Do(ctx, req, &pendingInvitations)
	if err != nil {
		return nil, resp, err
	}

	return pendingInvitations, resp, nil
}
