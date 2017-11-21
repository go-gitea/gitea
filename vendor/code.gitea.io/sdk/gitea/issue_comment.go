// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Comment represents a comment on a commit or issue
type Comment struct {
	ID       int64     `json:"id"`
	HTMLURL  string    `json:"html_url"`
	PRURL    string    `json:"pull_request_url"`
	IssueURL string    `json:"issue_url"`
	Poster   *User     `json:"user"`
	Body     string    `json:"body"`
	// swagger:strfmt date-time
	Created  time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated  time.Time `json:"updated_at"`
}

// ListIssueComments list comments on an issue.
func (c *Client) ListIssueComments(owner, repo string, index int64) ([]*Comment, error) {
	comments := make([]*Comment, 0, 10)
	return comments, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, index), nil, nil, &comments)
}

// ListRepoIssueComments list comments for a given repo.
func (c *Client) ListRepoIssueComments(owner, repo string) ([]*Comment, error) {
	comments := make([]*Comment, 0, 10)
	return comments, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues/comments", owner, repo), nil, nil, &comments)
}

// CreateIssueCommentOption options for creating a comment on an issue
type CreateIssueCommentOption struct {
	// required:true
	Body string `json:"body" binding:"Required"`
}

// CreateIssueComment create comment on an issue.
func (c *Client) CreateIssueComment(owner, repo string, index int64, opt CreateIssueCommentOption) (*Comment, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	comment := new(Comment)
	return comment, c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, index), jsonHeader, bytes.NewReader(body), comment)
}

// EditIssueCommentOption options for editing a comment
type EditIssueCommentOption struct {
	// required: true
	Body string `json:"body" binding:"Required"`
}

// EditIssueComment edits an issue comment.
func (c *Client) EditIssueComment(owner, repo string, index, commentID int64, opt EditIssueCommentOption) (*Comment, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	comment := new(Comment)
	return comment, c.getParsedResponse("PATCH", fmt.Sprintf("/repos/:%s/:%s/issues/%d/comments/%d", owner, repo, index, commentID), jsonHeader, bytes.NewReader(body), comment)
}

// DeleteIssueComment deletes an issue comment.
func (c *Client) DeleteIssueComment(owner, repo string, index, commentID int64) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/comments/%d", owner, repo, index, commentID), nil, nil)
	return err
}
