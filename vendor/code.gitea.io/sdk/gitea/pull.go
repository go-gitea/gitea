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

	"github.com/hashicorp/go-version"
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
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	prs := make([]*PullRequest, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/pulls", owner, repo))
	link.RawQuery = opt.QueryEncode()
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &prs)
	if c.checkServerVersionGreaterThanOrEqual(version1_14_0) != nil {
		for i := range prs {
			if err := fixPullHeadSha(c, prs[i]); err != nil {
				return prs, resp, err
			}
		}
	}
	return prs, resp, err
}

// GetPullRequest get information of one PR
func (c *Client) GetPullRequest(owner, repo string, index int64) (*PullRequest, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	pr := new(PullRequest)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, index), nil, nil, pr)
	if c.checkServerVersionGreaterThanOrEqual(version1_14_0) != nil {
		if err := fixPullHeadSha(c, pr); err != nil {
			return pr, resp, err
		}
	}
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
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
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
		if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
			return fmt.Errorf("can not change base gitea to old")
		}
	}
	return nil
}

// EditPullRequest modify pull request with PR id and options
func (c *Client) EditPullRequest(owner, repo string, index int64, opt EditPullRequestOption) (*PullRequest, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
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

var version1_11_5, _ = version.NewVersion("1.11.5")

// Validate the MergePullRequestOption struct
func (opt MergePullRequestOption) Validate(c *Client) error {
	if opt.Style == MergeStyleSquash {
		if err := c.checkServerVersionGreaterThanOrEqual(version1_11_5); err != nil {
			return err
		}
	}
	return nil
}

// MergePullRequest merge a PR to repository by PR id
func (c *Client) MergePullRequest(owner, repo string, index int64, opt MergePullRequestOption) (bool, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return false, nil, err
	}
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
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return false, nil, err
	}
	status, resp, err := c.getStatusCode("GET", fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, index), nil, nil)

	if err != nil {
		return false, resp, err
	}

	return status == 204, resp, nil
}

// getPullRequestDiffOrPatch gets the patch or diff file as bytes for a PR
func (c *Client) getPullRequestDiffOrPatch(owner, repo, kind string, index int64) ([]byte, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &kind); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_13_0); err != nil {
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

// ListPullRequestCommitsOptions options for listing pull requests
type ListPullRequestCommitsOptions struct {
	ListOptions
}

// ListPullRequestCommits list commits for a pull request
func (c *Client) ListPullRequestCommits(owner, repo string, index int64, opt ListPullRequestCommitsOptions) ([]*Commit, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/pulls/%d/commits", owner, repo, index))
	opt.setDefaults()
	commits := make([]*Commit, 0, opt.PageSize)
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse("GET", link.String(), nil, nil, &commits)
	return commits, resp, err
}

// fixPullHeadSha is a workaround for https://github.com/go-gitea/gitea/issues/12675
// When no head sha is available, this is because the branch got deleted in the base repo.
// pr.Head.Ref points in this case not to the head repo branch name, but the base repo ref,
// which stays available to resolve the commit sha. This is fixed for gitea >= 1.14.0
func fixPullHeadSha(client *Client, pr *PullRequest) error {
	if pr.Base != nil && pr.Base.Repository != nil && pr.Base.Repository.Owner != nil &&
		pr.Head != nil && pr.Head.Ref != "" && pr.Head.Sha == "" {
		owner := pr.Base.Repository.Owner.UserName
		repo := pr.Base.Repository.Name
		refs, _, err := client.GetRepoRefs(owner, repo, pr.Head.Ref)
		if err != nil {
			return err
		} else if len(refs) == 0 {
			return fmt.Errorf("unable to resolve PR ref '%s'", pr.Head.Ref)
		}
		pr.Head.Sha = refs[0].Object.SHA
	}
	return nil
}
