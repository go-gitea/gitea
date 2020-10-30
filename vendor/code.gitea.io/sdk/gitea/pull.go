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
	IsLocked  bool       `json:"is_locked"`
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
func (c *Client) ListRepoPullRequests(owner, repo string, opt ListPullRequestsOptions) ([]*PullRequest, *Response, error) {
	opt.setDefaults()
	prs := make([]*PullRequest, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/pulls", owner, repo))
	link.RawQuery = opt.QueryEncode()
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &prs)
	return prs, resp, err
}

// GetPullRequest get information of one PR
func (c *Client) GetPullRequest(owner, repo string, index int64) (*PullRequest, *Response, error) {
	pr := new(PullRequest)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, index), nil, nil, pr)
	return pr, resp, err
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
func (c *Client) CreatePullRequest(owner, repo string, opt CreatePullRequestOption) (*PullRequest, *Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	pr := new(PullRequest)
	resp, err := c.getParsedResponse("POST",
		fmt.Sprintf("/repos/%s/%s/pulls", owner, repo),
		jsonHeader, bytes.NewReader(body), pr)
	return pr, resp, err
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

// Validate the EditPullRequestOption struct
func (opt EditPullRequestOption) Validate(c *Client) error {
	if len(opt.Title) != 0 && len(strings.TrimSpace(opt.Title)) == 0 {
		return fmt.Errorf("title is empty")
	}
	if len(opt.Base) != 0 {
		if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
			return fmt.Errorf("can not change base gitea to old")
		}
	}
	return nil
}

// EditPullRequest modify pull request with PR id and options
func (c *Client) EditPullRequest(owner, repo string, index int64, opt EditPullRequestOption) (*PullRequest, *Response, error) {
	if err := opt.Validate(c); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	pr := new(PullRequest)
	resp, err := c.getParsedResponse("PATCH",
		fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, index),
		jsonHeader, bytes.NewReader(body), pr)
	return pr, resp, err
}

// MergePullRequestOption options when merging a pull request
type MergePullRequestOption struct {
	Style   MergeStyle `json:"Do"`
	Title   string     `json:"MergeTitleField"`
	Message string     `json:"MergeMessageField"`
}

// Validate the MergePullRequestOption struct
func (opt MergePullRequestOption) Validate(c *Client) error {
	if opt.Style == MergeStyleSquash {
		if err := c.CheckServerVersionConstraint(">=1.11.5"); err != nil {
			return err
		}
	}
	return nil
}

// MergePullRequest merge a PR to repository by PR id
func (c *Client) MergePullRequest(owner, repo string, index int64, opt MergePullRequestOption) (bool, *Response, error) {
	if err := opt.Validate(c); err != nil {
		return false, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return false, nil, err
	}
	status, resp, err := c.getStatusCode("POST", fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, index), jsonHeader, bytes.NewReader(body))
	if err != nil {
		return false, resp, err
	}
	return status == 200, resp, nil
}

// IsPullRequestMerged test if one PR is merged to one repository
func (c *Client) IsPullRequestMerged(owner, repo string, index int64) (bool, *Response, error) {
	status, resp, err := c.getStatusCode("GET", fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, index), nil, nil)

	if err != nil {
		return false, resp, err
	}

	return status == 204, resp, nil
}

// getPullRequestDiffOrPatch gets the patch or diff file as bytes for a PR
func (c *Client) getPullRequestDiffOrPatch(owner, repo, kind string, index int64) ([]byte, *Response, error) {
	if err := c.CheckServerVersionConstraint(">=1.13.0"); err != nil {
		r, _, err2 := c.GetRepo(owner, repo)
		if err2 != nil {
			return nil, nil, err
		}
		if r.Private {
			return nil, nil, err
		}
		return c.getWebResponse("GET", fmt.Sprintf("/%s/%s/pulls/%d.%s", owner, repo, index, kind), nil)
	}
	return c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/pulls/%d.%s", owner, repo, index, kind), nil, nil)
}

// GetPullRequestPatch gets the .patch file as bytes for a PR
func (c *Client) GetPullRequestPatch(owner, repo string, index int64) ([]byte, *Response, error) {
	return c.getPullRequestDiffOrPatch(owner, repo, "patch", index)
}

// GetPullRequestDiff gets the .diff file as bytes for a PR
func (c *Client) GetPullRequestDiff(owner, repo string, index int64) ([]byte, *Response, error) {
	return c.getPullRequestDiffOrPatch(owner, repo, "diff", index)
}
