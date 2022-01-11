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
	HTMLURL          string     `json:"html_url"`
	Index            int64      `json:"number"`
	Poster           *User      `json:"user"`
	OriginalAuthor   string     `json:"original_author"`
	OriginalAuthorID int64      `json:"original_author_id"`
	Title            string     `json:"title"`
	Body             string     `json:"body"`
	Ref              string     `json:"ref"`
	Labels           []*Label   `json:"labels"`
	Milestone        *Milestone `json:"milestone"`
	Assignees        []*User    `json:"assignees"`
	// Whether the issue is open or closed
	State       StateType        `json:"state"`
	IsLocked    bool             `json:"is_locked"`
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
	Since      time.Time
	Before     time.Time
	// filter by created by username
	CreatedBy string
	// filter by assigned to username
	AssignedBy string
	// filter by username mentioned
	MentionedBy string
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

	if !opt.Since.IsZero() {
		query.Add("since", opt.Since.Format(time.RFC3339))
	}
	if !opt.Before.IsZero() {
		query.Add("before", opt.Before.Format(time.RFC3339))
	}

	if len(opt.CreatedBy) > 0 {
		query.Add("created_by", opt.CreatedBy)
	}
	if len(opt.AssignedBy) > 0 {
		query.Add("assigned_by", opt.AssignedBy)
	}
	if len(opt.MentionedBy) > 0 {
		query.Add("mentioned_by", opt.MentionedBy)
	}

	return query.Encode()
}

// ListIssues returns all issues assigned the authenticated user
func (c *Client) ListIssues(opt ListIssueOption) ([]*Issue, *Response, error) {
	opt.setDefaults()
	issues := make([]*Issue, 0, opt.PageSize)

	link, _ := url.Parse("/repos/issues/search")
	link.RawQuery = opt.QueryEncode()
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &issues)
	if e := c.checkServerVersionGreaterThanOrEqual(version1_12_0); e != nil {
		for i := 0; i < len(issues); i++ {
			if issues[i].Repository != nil {
				issues[i].Repository.Owner = strings.Split(issues[i].Repository.FullName, "/")[0]
			}
		}
	}
	for i := range issues {
		c.issueBackwardsCompatibility(issues[i])
	}
	return issues, resp, err
}

// ListRepoIssues returns all issues for a given repository
func (c *Client) ListRepoIssues(owner, repo string, opt ListIssueOption) ([]*Issue, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	issues := make([]*Issue, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/issues", owner, repo))
	link.RawQuery = opt.QueryEncode()
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &issues)
	if e := c.checkServerVersionGreaterThanOrEqual(version1_12_0); e != nil {
		for i := 0; i < len(issues); i++ {
			if issues[i].Repository != nil {
				issues[i].Repository.Owner = strings.Split(issues[i].Repository.FullName, "/")[0]
			}
		}
	}
	for i := range issues {
		c.issueBackwardsCompatibility(issues[i])
	}
	return issues, resp, err
}

// GetIssue returns a single issue for a given repository
func (c *Client) GetIssue(owner, repo string, index int64) (*Issue, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	issue := new(Issue)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, index), nil, nil, issue)
	if e := c.checkServerVersionGreaterThanOrEqual(version1_12_0); e != nil && issue.Repository != nil {
		issue.Repository.Owner = strings.Split(issue.Repository.FullName, "/")[0]
	}
	c.issueBackwardsCompatibility(issue)
	return issue, resp, err
}

// CreateIssueOption options to create one issue
type CreateIssueOption struct {
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Ref       string     `json:"ref"`
	Assignees []string   `json:"assignees"`
	Deadline  *time.Time `json:"due_date"`
	// milestone id
	Milestone int64 `json:"milestone"`
	// list of label ids
	Labels []int64 `json:"labels"`
	Closed bool    `json:"closed"`
}

// Validate the CreateIssueOption struct
func (opt CreateIssueOption) Validate() error {
	if len(strings.TrimSpace(opt.Title)) == 0 {
		return fmt.Errorf("title is empty")
	}
	return nil
}

// CreateIssue create a new issue for a given repository
func (c *Client) CreateIssue(owner, repo string, opt CreateIssueOption) (*Issue, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	issue := new(Issue)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/issues", owner, repo),
		jsonHeader, bytes.NewReader(body), issue)
	c.issueBackwardsCompatibility(issue)
	return issue, resp, err
}

// EditIssueOption options for editing an issue
type EditIssueOption struct {
	Title          string     `json:"title"`
	Body           *string    `json:"body"`
	Ref            *string    `json:"ref"`
	Assignees      []string   `json:"assignees"`
	Milestone      *int64     `json:"milestone"`
	State          *StateType `json:"state"`
	Deadline       *time.Time `json:"due_date"`
	RemoveDeadline *bool      `json:"unset_due_date"`
}

// Validate the EditIssueOption struct
func (opt EditIssueOption) Validate() error {
	if len(opt.Title) != 0 && len(strings.TrimSpace(opt.Title)) == 0 {
		return fmt.Errorf("title is empty")
	}
	return nil
}

// EditIssue modify an existing issue for a given repository
func (c *Client) EditIssue(owner, repo string, index int64, opt EditIssueOption) (*Issue, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	issue := new(Issue)
	resp, err := c.getParsedResponse("PATCH",
		fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, index),
		jsonHeader, bytes.NewReader(body), issue)
	c.issueBackwardsCompatibility(issue)
	return issue, resp, err
}

func (c *Client) issueBackwardsCompatibility(issue *Issue) {
	if c.checkServerVersionGreaterThanOrEqual(version1_12_0) != nil {
		c.mutex.RLock()
		issue.HTMLURL = fmt.Sprintf("%s/%s/issues/%d", c.url, issue.Repository.FullName, issue.Index)
		c.mutex.RUnlock()
	}
}
