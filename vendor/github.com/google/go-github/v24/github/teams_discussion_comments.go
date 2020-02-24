// Copyright 2018 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// DiscussionComment represents a GitHub dicussion in a team.
type DiscussionComment struct {
	Author        *User      `json:"author,omitempty"`
	Body          *string    `json:"body,omitempty"`
	BodyHTML      *string    `json:"body_html,omitempty"`
	BodyVersion   *string    `json:"body_version,omitempty"`
	CreatedAt     *Timestamp `json:"created_at,omitempty"`
	LastEditedAt  *Timestamp `json:"last_edited_at,omitempty"`
	DiscussionURL *string    `json:"discussion_url,omitempty"`
	HTMLURL       *string    `json:"html_url,omitempty"`
	NodeID        *string    `json:"node_id,omitempty"`
	Number        *int       `json:"number,omitempty"`
	UpdatedAt     *Timestamp `json:"updated_at,omitempty"`
	URL           *string    `json:"url,omitempty"`
	Reactions     *Reactions `json:"reactions,omitempty"`
}

func (c DiscussionComment) String() string {
	return Stringify(c)
}

// DiscussionCommentListOptions specifies optional parameters to the
// TeamServices.ListComments method.
type DiscussionCommentListOptions struct {
	// Sorts the discussion comments by the date they were created.
	// Accepted values are asc and desc. Default is desc.
	Direction string `url:"direction,omitempty"`
}

// ListComments lists all comments on a team discussion.
// Authenticated user must grant read:discussion scope.
//
// GitHub API docs: https://developer.github.com/v3/teams/discussion_comments/#list-comments
func (s *TeamsService) ListComments(ctx context.Context, teamID int64, discussionNumber int, options *DiscussionCommentListOptions) ([]*DiscussionComment, *Response, error) {
	u := fmt.Sprintf("teams/%v/discussions/%v/comments", teamID, discussionNumber)
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

	var comments []*DiscussionComment
	resp, err := s.client.Do(ctx, req, &comments)
	if err != nil {
		return nil, resp, err
	}

	return comments, resp, nil
}

// GetComment gets a specific comment on a team discussion.
// Authenticated user must grant read:discussion scope.
//
// GitHub API docs: https://developer.github.com/v3/teams/discussion_comments/#get-a-single-comment
func (s *TeamsService) GetComment(ctx context.Context, teamID int64, discussionNumber, commentNumber int) (*DiscussionComment, *Response, error) {
	u := fmt.Sprintf("teams/%v/discussions/%v/comments/%v", teamID, discussionNumber, commentNumber)
	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeTeamDiscussionsPreview)

	discussionComment := &DiscussionComment{}
	resp, err := s.client.Do(ctx, req, discussionComment)
	if err != nil {
		return nil, resp, err
	}

	return discussionComment, resp, nil
}

// CreateComment creates a new discussion post on a team discussion.
// Authenticated user must grant write:discussion scope.
//
// GitHub API docs: https://developer.github.com/v3/teams/discussion_comments/#create-a-comment
func (s *TeamsService) CreateComment(ctx context.Context, teamID int64, discsusionNumber int, comment DiscussionComment) (*DiscussionComment, *Response, error) {
	u := fmt.Sprintf("teams/%v/discussions/%v/comments", teamID, discsusionNumber)
	req, err := s.client.NewRequest("POST", u, comment)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeTeamDiscussionsPreview)

	discussionComment := &DiscussionComment{}
	resp, err := s.client.Do(ctx, req, discussionComment)
	if err != nil {
		return nil, resp, err
	}

	return discussionComment, resp, nil
}

// EditComment edits the body text of a discussion comment.
// Authenticated user must grant write:discussion scope.
// User is allowed to edit body of a comment only.
//
// GitHub API docs: https://developer.github.com/v3/teams/discussion_comments/#edit-a-comment
func (s *TeamsService) EditComment(ctx context.Context, teamID int64, discussionNumber, commentNumber int, comment DiscussionComment) (*DiscussionComment, *Response, error) {
	u := fmt.Sprintf("teams/%v/discussions/%v/comments/%v", teamID, discussionNumber, commentNumber)
	req, err := s.client.NewRequest("PATCH", u, comment)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeTeamDiscussionsPreview)

	discussionComment := &DiscussionComment{}
	resp, err := s.client.Do(ctx, req, discussionComment)
	if err != nil {
		return nil, resp, err
	}

	return discussionComment, resp, nil
}

// DeleteComment deletes a comment on a team discussion.
// Authenticated user must grant write:discussion scope.
//
// GitHub API docs: https://developer.github.com/v3/teams/discussion_comments/#delete-a-comment
func (s *TeamsService) DeleteComment(ctx context.Context, teamID int64, discussionNumber, commentNumber int) (*Response, error) {
	u := fmt.Sprintf("teams/%v/discussions/%v/comments/%v", teamID, discussionNumber, commentNumber)
	req, err := s.client.NewRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeTeamDiscussionsPreview)

	return s.client.Do(ctx, req, nil)
}
