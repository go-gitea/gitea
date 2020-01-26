// Copyright 2016 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// ReactionsService provides access to the reactions-related functions in the
// GitHub API.
//
// GitHub API docs: https://developer.github.com/v3/reactions/
type ReactionsService service

// Reaction represents a GitHub reaction.
type Reaction struct {
	// ID is the Reaction ID.
	ID     *int64  `json:"id,omitempty"`
	User   *User   `json:"user,omitempty"`
	NodeID *string `json:"node_id,omitempty"`
	// Content is the type of reaction.
	// Possible values are:
	//     "+1", "-1", "laugh", "confused", "heart", "hooray".
	Content *string `json:"content,omitempty"`
}

// Reactions represents a summary of GitHub reactions.
type Reactions struct {
	TotalCount *int    `json:"total_count,omitempty"`
	PlusOne    *int    `json:"+1,omitempty"`
	MinusOne   *int    `json:"-1,omitempty"`
	Laugh      *int    `json:"laugh,omitempty"`
	Confused   *int    `json:"confused,omitempty"`
	Heart      *int    `json:"heart,omitempty"`
	Hooray     *int    `json:"hooray,omitempty"`
	URL        *string `json:"url,omitempty"`
}

func (r Reaction) String() string {
	return Stringify(r)
}

// ListCommentReactions lists the reactions for a commit comment.
//
// GitHub API docs: https://developer.github.com/v3/reactions/#list-reactions-for-a-commit-comment
func (s *ReactionsService) ListCommentReactions(ctx context.Context, owner, repo string, id int64, opt *ListOptions) ([]*Reaction, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/comments/%v/reactions", owner, repo, id)
	u, err := addOptions(u, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept headers when APIs fully launch.
	req.Header.Set("Accept", mediaTypeReactionsPreview)

	var m []*Reaction
	resp, err := s.client.Do(ctx, req, &m)
	if err != nil {
		return nil, resp, err
	}

	return m, resp, nil
}

// CreateCommentReaction creates a reaction for a commit comment.
// Note that if you have already created a reaction of type content, the
// previously created reaction will be returned with Status: 200 OK.
// The content should have one of the following values: "+1", "-1", "laugh", "confused", "heart", "hooray".
//
// GitHub API docs: https://developer.github.com/v3/reactions/#create-reaction-for-a-commit-comment
func (s ReactionsService) CreateCommentReaction(ctx context.Context, owner, repo string, id int64, content string) (*Reaction, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/comments/%v/reactions", owner, repo, id)

	body := &Reaction{Content: String(content)}
	req, err := s.client.NewRequest("POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept headers when APIs fully launch.
	req.Header.Set("Accept", mediaTypeReactionsPreview)

	m := &Reaction{}
	resp, err := s.client.Do(ctx, req, m)
	if err != nil {
		return nil, resp, err
	}

	return m, resp, nil
}

// ListIssueReactions lists the reactions for an issue.
//
// GitHub API docs: https://developer.github.com/v3/reactions/#list-reactions-for-an-issue
func (s *ReactionsService) ListIssueReactions(ctx context.Context, owner, repo string, number int, opt *ListOptions) ([]*Reaction, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/issues/%v/reactions", owner, repo, number)
	u, err := addOptions(u, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept headers when APIs fully launch.
	req.Header.Set("Accept", mediaTypeReactionsPreview)

	var m []*Reaction
	resp, err := s.client.Do(ctx, req, &m)
	if err != nil {
		return nil, resp, err
	}

	return m, resp, nil
}

// CreateIssueReaction creates a reaction for an issue.
// Note that if you have already created a reaction of type content, the
// previously created reaction will be returned with Status: 200 OK.
// The content should have one of the following values: "+1", "-1", "laugh", "confused", "heart", "hooray".
//
// GitHub API docs: https://developer.github.com/v3/reactions/#create-reaction-for-an-issue
func (s ReactionsService) CreateIssueReaction(ctx context.Context, owner, repo string, number int, content string) (*Reaction, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/issues/%v/reactions", owner, repo, number)

	body := &Reaction{Content: String(content)}
	req, err := s.client.NewRequest("POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept headers when APIs fully launch.
	req.Header.Set("Accept", mediaTypeReactionsPreview)

	m := &Reaction{}
	resp, err := s.client.Do(ctx, req, m)
	if err != nil {
		return nil, resp, err
	}

	return m, resp, nil
}

// ListIssueCommentReactions lists the reactions for an issue comment.
//
// GitHub API docs: https://developer.github.com/v3/reactions/#list-reactions-for-an-issue-comment
func (s *ReactionsService) ListIssueCommentReactions(ctx context.Context, owner, repo string, id int64, opt *ListOptions) ([]*Reaction, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/issues/comments/%v/reactions", owner, repo, id)
	u, err := addOptions(u, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept headers when APIs fully launch.
	req.Header.Set("Accept", mediaTypeReactionsPreview)

	var m []*Reaction
	resp, err := s.client.Do(ctx, req, &m)
	if err != nil {
		return nil, resp, err
	}

	return m, resp, nil
}

// CreateIssueCommentReaction creates a reaction for an issue comment.
// Note that if you have already created a reaction of type content, the
// previously created reaction will be returned with Status: 200 OK.
// The content should have one of the following values: "+1", "-1", "laugh", "confused", "heart", "hooray".
//
// GitHub API docs: https://developer.github.com/v3/reactions/#create-reaction-for-an-issue-comment
func (s ReactionsService) CreateIssueCommentReaction(ctx context.Context, owner, repo string, id int64, content string) (*Reaction, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/issues/comments/%v/reactions", owner, repo, id)

	body := &Reaction{Content: String(content)}
	req, err := s.client.NewRequest("POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept headers when APIs fully launch.
	req.Header.Set("Accept", mediaTypeReactionsPreview)

	m := &Reaction{}
	resp, err := s.client.Do(ctx, req, m)
	if err != nil {
		return nil, resp, err
	}

	return m, resp, nil
}

// ListPullRequestCommentReactions lists the reactions for a pull request review comment.
//
// GitHub API docs: https://developer.github.com/v3/reactions/#list-reactions-for-an-issue-comment
func (s *ReactionsService) ListPullRequestCommentReactions(ctx context.Context, owner, repo string, id int64, opt *ListOptions) ([]*Reaction, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/pulls/comments/%v/reactions", owner, repo, id)
	u, err := addOptions(u, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept headers when APIs fully launch.
	req.Header.Set("Accept", mediaTypeReactionsPreview)

	var m []*Reaction
	resp, err := s.client.Do(ctx, req, &m)
	if err != nil {
		return nil, resp, err
	}

	return m, resp, nil
}

// CreatePullRequestCommentReaction creates a reaction for a pull request review comment.
// Note that if you have already created a reaction of type content, the
// previously created reaction will be returned with Status: 200 OK.
// The content should have one of the following values: "+1", "-1", "laugh", "confused", "heart", "hooray".
//
// GitHub API docs: https://developer.github.com/v3/reactions/#create-reaction-for-an-issue-comment
func (s ReactionsService) CreatePullRequestCommentReaction(ctx context.Context, owner, repo string, id int64, content string) (*Reaction, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/pulls/comments/%v/reactions", owner, repo, id)

	body := &Reaction{Content: String(content)}
	req, err := s.client.NewRequest("POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept headers when APIs fully launch.
	req.Header.Set("Accept", mediaTypeReactionsPreview)

	m := &Reaction{}
	resp, err := s.client.Do(ctx, req, m)
	if err != nil {
		return nil, resp, err
	}

	return m, resp, nil
}

// ListTeamDiscussionReactions lists the reactions for a team discussion.
//
// GitHub API docs: https://developer.github.com/v3/reactions/#list-reactions-for-a-team-discussion
func (s *ReactionsService) ListTeamDiscussionReactions(ctx context.Context, teamID int64, discussionNumber int, opt *ListOptions) ([]*Reaction, *Response, error) {
	u := fmt.Sprintf("teams/%v/discussions/%v/reactions", teamID, discussionNumber)
	u, err := addOptions(u, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Accept", mediaTypeReactionsPreview)

	var m []*Reaction
	resp, err := s.client.Do(ctx, req, &m)
	if err != nil {
		return nil, resp, err
	}

	return m, resp, nil
}

// CreateTeamDiscussionReaction creates a reaction for a team discussion.
// The content should have one of the following values: "+1", "-1", "laugh", "confused", "heart", "hooray".
//
// GitHub API docs: https://developer.github.com/v3/reactions/#create-reaction-for-a-team-discussion
func (s *ReactionsService) CreateTeamDiscussionReaction(ctx context.Context, teamID int64, discussionNumber int, content string) (*Reaction, *Response, error) {
	u := fmt.Sprintf("teams/%v/discussions/%v/reactions", teamID, discussionNumber)

	body := &Reaction{Content: String(content)}
	req, err := s.client.NewRequest("POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Accept", mediaTypeReactionsPreview)

	m := &Reaction{}
	resp, err := s.client.Do(ctx, req, m)
	if err != nil {
		return nil, resp, err
	}

	return m, resp, nil
}

// ListTeamDiscussionCommentReactions lists the reactions for a team discussion comment.
//
// GitHub API docs: https://developer.github.com/v3/reactions/#list-reactions-for-a-team-discussion-comment
func (s *ReactionsService) ListTeamDiscussionCommentReactions(ctx context.Context, teamID int64, discussionNumber, commentNumber int, opt *ListOptions) ([]*Reaction, *Response, error) {
	u := fmt.Sprintf("teams/%v/discussions/%v/comments/%v/reactions", teamID, discussionNumber, commentNumber)
	u, err := addOptions(u, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Accept", mediaTypeReactionsPreview)

	var m []*Reaction
	resp, err := s.client.Do(ctx, req, &m)
	if err != nil {
		return nil, nil, err
	}
	return m, resp, nil
}

// CreateTeamDiscussionCommentReaction creates a reaction for a team discussion comment.
// The content should have one of the following values: "+1", "-1", "laugh", "confused", "heart", "hooray".
//
// GitHub API docs: https://developer.github.com/v3/reactions/#create-reaction-for-a-team-discussion-comment
func (s *ReactionsService) CreateTeamDiscussionCommentReaction(ctx context.Context, teamID int64, discussionNumber, commentNumber int, content string) (*Reaction, *Response, error) {
	u := fmt.Sprintf("teams/%v/discussions/%v/comments/%v/reactions", teamID, discussionNumber, commentNumber)

	body := &Reaction{Content: String(content)}
	req, err := s.client.NewRequest("POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Accept", mediaTypeReactionsPreview)

	m := &Reaction{}
	resp, err := s.client.Do(ctx, req, m)
	if err != nil {
		return nil, resp, err
	}

	return m, resp, nil
}

// DeleteReaction deletes a reaction.
//
// GitHub API docs: https://developer.github.com/v3/reaction/reactions/#delete-a-reaction-archive
func (s *ReactionsService) DeleteReaction(ctx context.Context, id int64) (*Response, error) {
	u := fmt.Sprintf("reactions/%v", id)

	req, err := s.client.NewRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeReactionsPreview)

	return s.client.Do(ctx, req, nil)
}
