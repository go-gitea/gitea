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

// Issue represents an issue in a repository
// swagger:model
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
	Assignees []*User    `json:"assignees"`
	// Whether the issue is open or closed
	//
	// type: string
	// enum: open,closed
	State    StateType `json:"state"`
	Comments int       `json:"comments"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	Closed *time.Time `json:"closed_at"`
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`

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
	// required:true
	Title string `json:"title" binding:"Required"`
	Body  string `json:"body"`
	// username of assignee
	Assignee  string   `json:"assignee"`
	Assignees []string `json:"assignees"`
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`
	// milestone id
	Milestone int64 `json:"milestone"`
	// list of label ids
	Labels []int64 `json:"labels"`
	Closed bool    `json:"closed"`
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

// EditIssueOption options for editing an issue
type EditIssueOption struct {
	Title     string   `json:"title"`
	Body      *string  `json:"body"`
	Assignee  *string  `json:"assignee"`
	Assignees []string `json:"assignees"`
	Milestone *int64   `json:"milestone"`
	State     *string  `json:"state"`
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`
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

// StartIssueStopWatch starts a stopwatch for an existing issue for a given
// repository
func (c *Client) StartIssueStopWatch(owner, repo string, index int64) error {
	_, err := c.getResponse("POST", fmt.Sprintf("/repos/%s/%s/issues/%d/stopwatch/start", owner, repo, index),
		jsonHeader, nil)
	return err
}

// StopIssueStopWatch stops an existing stopwatch for an issue in a given
// repository
func (c *Client) StopIssueStopWatch(owner, repo string, index int64) error {
	_, err := c.getResponse("POST", fmt.Sprintf("/repos/%s/%s/issues/%d/stopwatch/stop", owner, repo, index),
		jsonHeader, nil)
	return err
}

// EditDeadlineOption options for creating a deadline
type EditDeadlineOption struct {
	// required:true
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`
}

// IssueDeadline represents an issue deadline
// swagger:model
type IssueDeadline struct {
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`
}

// EditPriorityOption options for updating priority
type EditPriorityOption struct {
	// required:true
	Priority int `json:"priority"`
}
