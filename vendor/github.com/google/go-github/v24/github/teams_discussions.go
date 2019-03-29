// Copyright 2018 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// TeamDiscussion represents a GitHub dicussion in a team.
type TeamDiscussion struct {
	Author        *User      `json:"author,omitempty"`
	Body          *string    `json:"body,omitempty"`
	BodyHTML      *string    `json:"body_html,omitempty"`
	BodyVersion   *string    `json:"body_version,omitempty"`
	CommentsCount *int       `json:"comments_count,omitempty"`
	CommentsURL   *string    `json:"comments_url,omitempty"`
	CreatedAt     *Timestamp `json:"created_at,omitempty"`
	LastEditedAt  *Timestamp `json:"last_edited_at,omitempty"`
	HTMLURL       *string    `json:"html_url,omitempty"`
	NodeID        *string    `json:"node_id,omitempty"`
	Number        *int       `json:"number,omitempty"`
	Pinned        *bool      `json:"pinned,omitempty"`
	Private       *bool      `json:"private,omitempty"`
	TeamURL       *string    `json:"team_url,omitempty"`
	Title         *string    `json:"title,omitempty"`
	UpdatedAt     *Timestamp `json:"updated_at,omitempty"`
	URL           *string    `json:"url,omitempty"`
	Reactions     *Reactions `json:"reactions,omitempty"`
}

func (d TeamDiscussion) String() string {
	return Stringify(d)
}

// DiscussionListOptions specifies optional parameters to the
// TeamServices.ListDiscussions method.
type DiscussionListOptions struct {
	// Sorts the discussion by the date they were created.
	// Accepted values are asc and desc. Default is desc.
	Direction string `url:"direction,omitempty"`
}

// ListDiscussions lists all discussions on team's page.
// Authenticated user must grant read:discussion scope.
//
// GitHub API docs: https://developer.github.com/v3/teams/discussions/#list-discussions
func (s *TeamsService) ListDiscussions(ctx context.Context, teamID int64, options *DiscussionListOptions) ([]*TeamDiscussion, *Response, error) {
	u := fmt.Sprintf("teams/%v/discussions", teamID)
	u, err := addOptions(u, options)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeTeamDiscussionsPreview)

	var teamDiscussions []*TeamDiscussion
	resp, err := s.client.Do(ctx, req, &teamDiscussions)
	if err != nil {
		return nil, resp, err
	}

	return teamDiscussions, resp, nil
}

// GetDiscussion gets a specific discussion on a team's page.
// Authenticated user must grant read:discussion scope.
//
// GitHub API docs: https://developer.github.com/v3/teams/discussions/#get-a-single-discussion
func (s *TeamsService) GetDiscussion(ctx context.Context, teamID int64, discussionNumber int) (*TeamDiscussion, *Response, error) {
	u := fmt.Sprintf("teams/%v/discussions/%v", teamID, discussionNumber)
	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeTeamDiscussionsPreview)

	teamDiscussion := &TeamDiscussion{}
	resp, err := s.client.Do(ctx, req, teamDiscussion)
	if err != nil {
		return nil, resp, err
	}

	return teamDiscussion, resp, nil
}

// CreateDiscussion creates a new discussion post on a team's page.
// Authenticated user must grant write:discussion scope.
//
// GitHub API docs: https://developer.github.com/v3/teams/discussions/#create-a-discussion
func (s *TeamsService) CreateDiscussion(ctx context.Context, teamID int64, discussion TeamDiscussion) (*TeamDiscussion, *Response, error) {
	u := fmt.Sprintf("teams/%v/discussions", teamID)
	req, err := s.client.NewRequest("POST", u, discussion)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeTeamDiscussionsPreview)

	teamDiscussion := &TeamDiscussion{}
	resp, err := s.client.Do(ctx, req, teamDiscussion)
	if err != nil {
		return nil, resp, err
	}

	return teamDiscussion, resp, nil
}

// EditDiscussion edits the title and body text of a discussion post.
// Authenticated user must grant write:discussion scope.
// User is allowed to change Title and Body of a discussion only.
//
// GitHub API docs: https://developer.github.com/v3/teams/discussions/#edit-a-discussion
func (s *TeamsService) EditDiscussion(ctx context.Context, teamID int64, discussionNumber int, discussion TeamDiscussion) (*TeamDiscussion, *Response, error) {
	u := fmt.Sprintf("teams/%v/discussions/%v", teamID, discussionNumber)
	req, err := s.client.NewRequest("PATCH", u, discussion)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeTeamDiscussionsPreview)

	teamDiscussion := &TeamDiscussion{}
	resp, err := s.client.Do(ctx, req, teamDiscussion)
	if err != nil {
		return nil, resp, err
	}

	return teamDiscussion, resp, nil
}

// DeleteDiscussion deletes a discussion from team's page.
// Authenticated user must grant write:discussion scope.
//
// GitHub API docs: https://developer.github.com/v3/teams/discussions/#delete-a-discussion
func (s *TeamsService) DeleteDiscussion(ctx context.Context, teamID int64, discussionNumber int) (*Response, error) {
	u := fmt.Sprintf("teams/%v/discussions/%v", teamID, discussionNumber)
	req, err := s.client.NewRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeTeamDiscussionsPreview)

	return s.client.Do(ctx, req, nil)
}
