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
	"time"
)

// PRBranchInfo information about a branch
type PRBranchInfo struct {
	Name       string      `json:"label"`
	Ref        string      `json:"ref"`
	Sha        string      `json:"sha"`
	RepoID     int64       `json:"repo_id"`
	Repository *Repository `json:"repo"`
}

// PullRequest represents a pull request
type PullRequest struct {
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
	State     StateType  `json:"state"`
	Comments  int        `json:"comments"`

	HTMLURL  string `json:"html_url"`
	DiffURL  string `json:"diff_url"`
	PatchURL string `json:"patch_url"`

	Mergeable      bool       `json:"mergeable"`
	HasMerged      bool       `json:"merged"`
	Merged         *time.Time `json:"merged_at"`
	MergedCommitID *string    `json:"merge_commit_sha"`
	MergedBy       *User      `json:"merged_by"`

	Base      *PRBranchInfo `json:"base"`
	Head      *PRBranchInfo `json:"head"`
	MergeBase string        `json:"merge_base"`

	Deadline *time.Time `json:"due_date"`
	Created  *time.Time `json:"created_at"`
	Updated  *time.Time `json:"updated_at"`
	Closed   *time.Time `json:"closed_at"`
}

// ListPullRequestsOptions options for listing pull requests
type ListPullRequestsOptions struct {
	ListOptions
	State StateType `json:"state"`
	// oldest, recentupdate, leastupdate, mostcomment, leastcomment, priority
	Sort      string
	Milestone int64
}

// MergeStyle is used specify how a pull is merged
type MergeStyle string

const (
	// MergeStyleMerge merge pull as usual
	MergeStyleMerge MergeStyle = "merge"
	// MergeStyleRebase rebase pull
	MergeStyleRebase MergeStyle = "rebase"
	// MergeStyleRebaseMerge rebase and merge pull
	MergeStyleRebaseMerge MergeStyle = "rebase-merge"
	// MergeStyleSquash squash and merge pull
	MergeStyleSquash MergeStyle = "squash"
)

// QueryEncode turns options into querystring argument
func (opt *ListPullRequestsOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if len(opt.State) > 0 {
		query.Add("state", string(opt.State))
	}
	if len(opt.Sort) > 0 {
		query.Add("sort", opt.Sort)
	}
	if opt.Milestone > 0 {
		query.Add("milestone", fmt.Sprintf("%d", opt.Milestone))
	}
	return query.Encode()
}

// ListRepoPullRequests list PRs of one repository
func (c *Client) ListRepoPullRequests(owner, repo string, opt ListPullRequestsOptions) ([]*PullRequest, error) {
	opt.setDefaults()
	prs := make([]*PullRequest, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/pulls", owner, repo))
	link.RawQuery = opt.QueryEncode()
	return prs, c.getParsedResponse("GET", link.String(), jsonHeader, nil, &prs)
}

// GetPullRequest get information of one PR
func (c *Client) GetPullRequest(owner, repo string, index int64) (*PullRequest, error) {
	pr := new(PullRequest)
	return pr, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, index), nil, nil, pr)
}

// CreatePullRequestOption options when creating a pull request
type CreatePullRequestOption struct {
	Head      string     `json:"head"`
	Base      string     `json:"base"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Assignee  string     `json:"assignee"`
	Assignees []string   `json:"assignees"`
	Milestone int64      `json:"milestone"`
	Labels    []int64    `json:"labels"`
	Deadline  *time.Time `json:"due_date"`
}

// CreatePullRequest create pull request with options
func (c *Client) CreatePullRequest(owner, repo string, opt CreatePullRequestOption) (*PullRequest, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	pr := new(PullRequest)
	return pr, c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/pulls", owner, repo),
		jsonHeader, bytes.NewReader(body), pr)
}

// EditPullRequestOption options when modify pull request
type EditPullRequestOption struct {
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Base      string     `json:"base"`
	Assignee  string     `json:"assignee"`
	Assignees []string   `json:"assignees"`
	Milestone int64      `json:"milestone"`
	Labels    []int64    `json:"labels"`
	State     *StateType `json:"state"`
	Deadline  *time.Time `json:"due_date"`
}

// EditPullRequest modify pull request with PR id and options
func (c *Client) EditPullRequest(owner, repo string, index int64, opt EditPullRequestOption) (*PullRequest, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	pr := new(PullRequest)
	return pr, c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, index),
		jsonHeader, bytes.NewReader(body), pr)
}

// MergePullRequestOption options when merging a pull request
type MergePullRequestOption struct {
	Style   MergeStyle `json:"Do"`
	Title   string     `json:"MergeTitleField"`
	Message string     `json:"MergeMessageField"`
}

// MergePullRequest merge a PR to repository by PR id
func (c *Client) MergePullRequest(owner, repo string, index int64, opt MergePullRequestOption) (bool, error) {
	if opt.Style == MergeStyleSquash {
		if err := c.CheckServerVersionConstraint(">=1.11.5"); err != nil {
			return false, err
		}
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return false, err
	}
	status, err := c.getStatusCode("POST", fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, index), jsonHeader, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	return status == 200, nil
}

// IsPullRequestMerged test if one PR is merged to one repository
func (c *Client) IsPullRequestMerged(owner, repo string, index int64) (bool, error) {
	statusCode, err := c.getStatusCode("GET", fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, index), nil, nil)

	if err != nil {
		return false, err
	}

	return statusCode == 204, nil
}
