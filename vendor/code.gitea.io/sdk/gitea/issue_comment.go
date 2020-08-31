// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// Comment represents a comment on a commit or issue
type Comment struct {
	ID               int64     `json:"id"`
	HTMLURL          string    `json:"html_url"`
	PRURL            string    `json:"pull_request_url"`
	IssueURL         string    `json:"issue_url"`
	Poster           *User     `json:"user"`
	OriginalAuthor   string    `json:"original_author"`
	OriginalAuthorID int64     `json:"original_author_id"`
	Body             string    `json:"body"`
	Created          time.Time `json:"created_at"`
	Updated          time.Time `json:"updated_at"`
}

// ListIssueCommentOptions list comment options
type ListIssueCommentOptions struct {
	ListOptions
	Since  time.Time
	Before time.Time
}

// QueryEncode turns options into querystring argument
func (opt *ListIssueCommentOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if !opt.Since.IsZero() {
		query.Add("since", opt.Since.Format(time.RFC3339))
	}
	if !opt.Before.IsZero() {
		query.Add("before", opt.Before.Format(time.RFC3339))
	}
	return query.Encode()
}

// ListIssueComments list comments on an issue.
func (c *Client) ListIssueComments(owner, repo string, index int64, opt ListIssueCommentOptions) ([]*Comment, error) {
	opt.setDefaults()
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, index))
	link.RawQuery = opt.QueryEncode()
	comments := make([]*Comment, 0, opt.PageSize)
	return comments, c.getParsedResponse("GET", link.String(), nil, nil, &comments)
}

// ListRepoIssueComments list comments for a given repo.
func (c *Client) ListRepoIssueComments(owner, repo string, opt ListIssueCommentOptions) ([]*Comment, error) {
	opt.setDefaults()
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/issues/comments", owner, repo))
	link.RawQuery = opt.QueryEncode()
	comments := make([]*Comment, 0, opt.PageSize)
	return comments, c.getParsedResponse("GET", link.String(), nil, nil, &comments)
}

// GetIssueComment get a comment for a given repo by id.
func (c *Client) GetIssueComment(owner, repo string, id int64) (*Comment, error) {
	comment := new(Comment)
	if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
		return comment, err
	}
	return comment, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues/comments/%d", owner, repo, id), nil, nil, &comment)
}

// CreateIssueCommentOption options for creating a comment on an issue
type CreateIssueCommentOption struct {
	Body string `json:"body"`
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
	Body string `json:"body"`
}

// EditIssueComment edits an issue comment.
func (c *Client) EditIssueComment(owner, repo string, commentID int64, opt EditIssueCommentOption) (*Comment, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	comment := new(Comment)
	return comment, c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s/issues/comments/%d", owner, repo, commentID), jsonHeader, bytes.NewReader(body), comment)
}

// DeleteIssueComment deletes an issue comment.
func (c *Client) DeleteIssueComment(owner, repo string, commentID int64) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/issues/comments/%d", owner, repo, commentID), nil, nil)
	return err
}
