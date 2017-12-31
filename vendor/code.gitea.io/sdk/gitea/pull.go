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
	State     StateType  `json:"state"`
	Comments  int        `json:"comments"`

	HTMLURL  string `json:"html_url"`
	DiffURL  string `json:"diff_url"`
	PatchURL string `json:"patch_url"`

	Mergeable      bool       `json:"mergeable"`
	HasMerged      bool       `json:"merged"`
	// swagger:strfmt date-time
	Merged         *time.Time `json:"merged_at"`
	MergedCommitID *string    `json:"merge_commit_sha"`
	MergedBy       *User      `json:"merged_by"`

	Base      *PRBranchInfo `json:"base"`
	Head      *PRBranchInfo `json:"head"`
	MergeBase string        `json:"merge_base"`

	// swagger:strfmt date-time
	Created *time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated *time.Time `json:"updated_at"`
}

// PRBranchInfo information about a branch
type PRBranchInfo struct {
	Name       string      `json:"label"`
	Ref        string      `json:"ref"`
	Sha        string      `json:"sha"`
	RepoID     int64       `json:"repo_id"`
	Repository *Repository `json:"repo"`
}

// ListPullRequestsOptions options for listing pull requests
type ListPullRequestsOptions struct {
	Page  int    `json:"page"`
	State string `json:"state"`
}

// ListRepoPullRequests list PRs of one repository
func (c *Client) ListRepoPullRequests(owner, repo string, opt ListPullRequestsOptions) ([]*PullRequest, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	prs := make([]*PullRequest, 0, 10)
	return prs, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/pulls", owner, repo), jsonHeader, bytes.NewReader(body), &prs)
}

// GetPullRequest get information of one PR
func (c *Client) GetPullRequest(owner, repo string, index int64) (*PullRequest, error) {
	pr := new(PullRequest)
	return pr, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, index), nil, nil, pr)
}

// CreatePullRequestOption options when creating a pull request
type CreatePullRequestOption struct {
	Head      string  `json:"head" binding:"Required"`
	Base      string  `json:"base" binding:"Required"`
	Title     string  `json:"title" binding:"Required"`
	Body      string  `json:"body"`
	Assignee  string  `json:"assignee"`
	Milestone int64   `json:"milestone"`
	Labels    []int64 `json:"labels"`
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
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	Assignee  string  `json:"assignee"`
	Milestone int64   `json:"milestone"`
	Labels    []int64 `json:"labels"`
	State     *string `json:"state"`
}

// EditPullRequest modify pull request with PR id and options
func (c *Client) EditPullRequest(owner, repo string, index int64, opt EditPullRequestOption) (*PullRequest, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	pr := new(PullRequest)
	return pr, c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, index),
		jsonHeader, bytes.NewReader(body), pr)
}

// MergePullRequest merge a PR to repository by PR id
func (c *Client) MergePullRequest(owner, repo string, index int64) error {
	_, err := c.getResponse("POST", fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, index), nil, nil)
	return err
}

// IsPullRequestMerged test if one PR is merged to one repository
func (c *Client) IsPullRequestMerged(owner, repo string, index int64) (bool, error) {
	statusCode, err := c.getStatusCode("GET", fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, index), nil, nil)

	if err != nil {
		return false, err
	}

	return statusCode == 204, nil

}
