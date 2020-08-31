// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// PullRequestMeta PR info if an issue is a PR
type PullRequestMeta struct {
	HasMerged bool       `json:"merged"`
	Merged    *time.Time `json:"merged_at"`
}

// RepositoryMeta basic repository information
type RepositoryMeta struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Owner    string `json:"owner"`
	FullName string `json:"full_name"`
}

// Issue represents an issue in a repository
type Issue struct {
	ID               int64      `json:"id"`
	URL              string     `json:"url"`
	Index            int64      `json:"number"`
	Poster           *User      `json:"user"`
	OriginalAuthor   string     `json:"original_author"`
	OriginalAuthorID int64      `json:"original_author_id"`
	Title            string     `json:"title"`
	Body             string     `json:"body"`
	Labels           []*Label   `json:"labels"`
	Milestone        *Milestone `json:"milestone"`
	Assignee         *User      `json:"assignee"`
	Assignees        []*User    `json:"assignees"`
	// Whether the issue is open or closed
	State       StateType        `json:"state"`
	Comments    int              `json:"comments"`
	Created     time.Time        `json:"created_at"`
	Updated     time.Time        `json:"updated_at"`
	Closed      *time.Time       `json:"closed_at"`
	Deadline    *time.Time       `json:"due_date"`
	PullRequest *PullRequestMeta `json:"pull_request"`
	Repository  *RepositoryMeta  `json:"repository"`
}

// ListIssueOption list issue options
type ListIssueOption struct {
	ListOptions
	State      StateType
	Type       IssueType
	Labels     []string
	Milestones []string
	KeyWord    string
}

// StateType issue state type
type StateType string

const (
	// StateOpen pr/issue is opend
	StateOpen StateType = "open"
	// StateClosed pr/issue is closed
	StateClosed StateType = "closed"
	// StateAll is all
	StateAll StateType = "all"
)

// IssueType is issue a pull or only an issue
type IssueType string

const (
	// IssueTypeAll pr and issue
	IssueTypeAll IssueType = ""
	// IssueTypeIssue only issues
	IssueTypeIssue IssueType = "issues"
	// IssueTypePull only pulls
	IssueTypePull IssueType = "pulls"
)

// QueryEncode turns options into querystring argument
func (opt *ListIssueOption) QueryEncode() string {
	query := opt.getURLQuery()

	if len(opt.State) > 0 {
		query.Add("state", string(opt.State))
	}

	if len(opt.Labels) > 0 {
		query.Add("labels", strings.Join(opt.Labels, ","))
	}

	if len(opt.KeyWord) > 0 {
		query.Add("q", opt.KeyWord)
	}

	query.Add("type", string(opt.Type))

	if len(opt.Milestones) > 0 {
		query.Add("milestones", strings.Join(opt.Milestones, ","))
	}

	return query.Encode()
}

// ListIssues returns all issues assigned the authenticated user
func (c *Client) ListIssues(opt ListIssueOption) ([]*Issue, error) {
	opt.setDefaults()
	issues := make([]*Issue, 0, opt.PageSize)

	link, _ := url.Parse("/repos/issues/search")
	link.RawQuery = opt.QueryEncode()
	err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &issues)
	if e := c.CheckServerVersionConstraint(">=1.12.0"); e != nil {
		for i := 0; i < len(issues); i++ {
			if issues[i].Repository != nil {
				issues[i].Repository.Owner = strings.Split(issues[i].Repository.FullName, "/")[0]
			}
		}
	}
	return issues, err
}

// ListRepoIssues returns all issues for a given repository
func (c *Client) ListRepoIssues(owner, repo string, opt ListIssueOption) ([]*Issue, error) {
	opt.setDefaults()
	issues := make([]*Issue, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/issues", owner, repo))
	link.RawQuery = opt.QueryEncode()
	err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &issues)
	if e := c.CheckServerVersionConstraint(">=1.12.0"); e != nil {
		for i := 0; i < len(issues); i++ {
			if issues[i].Repository != nil {
				issues[i].Repository.Owner = strings.Split(issues[i].Repository.FullName, "/")[0]
			}
		}
	}
	return issues, err
}

// GetIssue returns a single issue for a given repository
func (c *Client) GetIssue(owner, repo string, index int64) (*Issue, error) {
	issue := new(Issue)
	err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, index), nil, nil, issue)
	if e := c.CheckServerVersionConstraint(">=1.12.0"); e != nil && issue.Repository != nil {
		issue.Repository.Owner = strings.Split(issue.Repository.FullName, "/")[0]
	}
	return issue, err
}

// CreateIssueOption options to create one issue
type CreateIssueOption struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	// username of assignee
	Assignee  string     `json:"assignee"`
	Assignees []string   `json:"assignees"`
	Deadline  *time.Time `json:"due_date"`
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
	Title     string     `json:"title"`
	Body      *string    `json:"body"`
	Assignee  *string    `json:"assignee"`
	Assignees []string   `json:"assignees"`
	Milestone *int64     `json:"milestone"`
	State     *StateType `json:"state"`
	Deadline  *time.Time `json:"due_date"`
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
