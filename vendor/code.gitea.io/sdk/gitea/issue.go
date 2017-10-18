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

// StateType issue state type
type StateType string

const (
	// StateOpen pr is opend
	StateOpen StateType = "open"
	// StateClosed pr is closed
	StateClosed StateType = "closed"
)

// PullRequestMeta PR info if an issue is a PR
type PullRequestMeta struct {
	HasMerged bool       `json:"merged"`
	Merged    *time.Time `json:"merged_at"`
}

// Issue an issue to a repository
type Issue struct {
	ID        int64      `json:"id"`
	URL       string     `json:"url"`
	Index     int64      `json:"number"`
	Poster    *User      `json:"user"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Labels    []*Label   `json:"labels"`
	Milestone *Milestone `json:"milestone"`
	Assignee  *User      `json:"assignee"`
	State     StateType  `json:"state"`
	Comments  int        `json:"comments"`
	Created   time.Time  `json:"created_at"`
	Updated   time.Time  `json:"updated_at"`

	PullRequest *PullRequestMeta `json:"pull_request"`
}

// ListIssueOption list issue options
type ListIssueOption struct {
	Page  int
	State string
}

// ListIssues returns all issues assigned the authenticated user
func (c *Client) ListIssues(opt ListIssueOption) ([]*Issue, error) {
	issues := make([]*Issue, 0, 10)
	return issues, c.getParsedResponse("GET", fmt.Sprintf("/issues?page=%d", opt.Page), nil, nil, &issues)
}

// ListUserIssues returns all issues assigned to the authenticated user
func (c *Client) ListUserIssues(opt ListIssueOption) ([]*Issue, error) {
	issues := make([]*Issue, 0, 10)
	return issues, c.getParsedResponse("GET", fmt.Sprintf("/user/issues?page=%d", opt.Page), nil, nil, &issues)
}

// ListRepoIssues returns all issues for a given repository
func (c *Client) ListRepoIssues(owner, repo string, opt ListIssueOption) ([]*Issue, error) {
	issues := make([]*Issue, 0, 10)
	return issues, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues?page=%d", owner, repo, opt.Page), nil, nil, &issues)
}

// GetIssue returns a single issue for a given repository
func (c *Client) GetIssue(owner, repo string, index int64) (*Issue, error) {
	issue := new(Issue)
	return issue, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, index), nil, nil, issue)
}

// CreateIssueOption options to create one issue
type CreateIssueOption struct {
	Title     string  `json:"title" binding:"Required"`
	Body      string  `json:"body"`
	Assignee  string  `json:"assignee"`
	Milestone int64   `json:"milestone"`
	Labels    []int64 `json:"labels"`
	Closed    bool    `json:"closed"`
}

// CreateIssue create a new issue for a given repository
func (c *Client) CreateIssue(owner, repo string, opt CreateIssueOption) (*Issue, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	issue := new(Issue)
	return issue, c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/issues", owner, repo),
		jsonHeader, bytes.NewReader(body), issue)
}

// EditIssueOption edit issue options
type EditIssueOption struct {
	Title     string  `json:"title"`
	Body      *string `json:"body"`
	Assignee  *string `json:"assignee"`
	Milestone *int64  `json:"milestone"`
	State     *string `json:"state"`
}

// EditIssue modify an existing issue for a given repository
func (c *Client) EditIssue(owner, repo string, index int64, opt EditIssueOption) (*Issue, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	issue := new(Issue)
	return issue, c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, index),
		jsonHeader, bytes.NewReader(body), issue)
}
