// Copyright 2020 The Gitea Authors. All rights reserved.
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

// ReviewStateType review state type
type ReviewStateType string

const (
	// ReviewStateApproved pr is approved
	ReviewStateApproved ReviewStateType = "APPROVED"
	// ReviewStatePending pr state is pending
	ReviewStatePending ReviewStateType = "PENDING"
	// ReviewStateComment is a comment review
	ReviewStateComment ReviewStateType = "COMMENT"
	// ReviewStateRequestChanges changes for pr are requested
	ReviewStateRequestChanges ReviewStateType = "REQUEST_CHANGES"
	// ReviewStateRequestReview review is requested from user
	ReviewStateRequestReview ReviewStateType = "REQUEST_REVIEW"
	// ReviewStateUnknown state of pr is unknown
	ReviewStateUnknown ReviewStateType = ""
)

// PullReview represents a pull request review
type PullReview struct {
	ID                int64           `json:"id"`
	Reviewer          *User           `json:"user"`
	State             ReviewStateType `json:"state"`
	Body              string          `json:"body"`
	CommitID          string          `json:"commit_id"`
	Stale             bool            `json:"stale"`
	Official          bool            `json:"official"`
	CodeCommentsCount int             `json:"comments_count"`
	// swagger:strfmt date-time
	Submitted time.Time `json:"submitted_at"`

	HTMLURL     string `json:"html_url"`
	HTMLPullURL string `json:"pull_request_url"`
}

// PullReviewComment represents a comment on a pull request review
type PullReviewComment struct {
	ID       int64  `json:"id"`
	Body     string `json:"body"`
	Reviewer *User  `json:"user"`
	ReviewID int64  `json:"pull_request_review_id"`

	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`

	Path         string `json:"path"`
	CommitID     string `json:"commit_id"`
	OrigCommitID string `json:"original_commit_id"`
	DiffHunk     string `json:"diff_hunk"`
	LineNum      uint64 `json:"position"`
	OldLineNum   uint64 `json:"original_position"`

	HTMLURL     string `json:"html_url"`
	HTMLPullURL string `json:"pull_request_url"`
}

// CreatePullReviewOptions are options to create a pull review
type CreatePullReviewOptions struct {
	State    ReviewStateType           `json:"event"`
	Body     string                    `json:"body"`
	CommitID string                    `json:"commit_id"`
	Comments []CreatePullReviewComment `json:"comments"`
}

// CreatePullReviewComment represent a review comment for creation api
type CreatePullReviewComment struct {
	// the tree path
	Path string `json:"path"`
	Body string `json:"body"`
	// if comment to old file line or 0
	OldLineNum int64 `json:"old_position"`
	// if comment to new file line or 0
	NewLineNum int64 `json:"new_position"`
}

// SubmitPullReviewOptions are options to submit a pending pull review
type SubmitPullReviewOptions struct {
	State ReviewStateType `json:"event"`
	Body  string          `json:"body"`
}

// ListPullReviewsOptions options for listing PullReviews
type ListPullReviewsOptions struct {
	ListOptions
}

// ListPullReviews lists all reviews of a pull request
func (c *Client) ListPullReviews(owner, repo string, index int64, opt ListPullReviewsOptions) ([]*PullReview, error) {
	if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
		return nil, err
	}
	opt.setDefaults()
	rs := make([]*PullReview, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", owner, repo, index))
	link.RawQuery = opt.ListOptions.getURLQuery().Encode()

	return rs, c.getParsedResponse("GET", link.String(), jsonHeader, nil, &rs)
}

// GetPullReview gets a specific review of a pull request
func (c *Client) GetPullReview(owner, repo string, index, id int64) (*PullReview, error) {
	if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
		return nil, err
	}

	r := new(PullReview)
	return r, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews/%d", owner, repo, index, id), jsonHeader, nil, &r)
}

// ListPullReviewsCommentsOptions options for listing PullReviewsComments
type ListPullReviewsCommentsOptions struct {
	ListOptions
}

// ListPullReviewComments lists all comments of a pull request review
func (c *Client) ListPullReviewComments(owner, repo string, index, id int64, opt ListPullReviewsCommentsOptions) ([]*PullReviewComment, error) {
	if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
		return nil, err
	}
	opt.setDefaults()
	rcl := make([]*PullReviewComment, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews/%d/comments", owner, repo, index, id))
	link.RawQuery = opt.ListOptions.getURLQuery().Encode()

	return rcl, c.getParsedResponse("GET", link.String(), jsonHeader, nil, &rcl)
}

// DeletePullReview delete a specific review from a pull request
func (c *Client) DeletePullReview(owner, repo string, index, id int64) error {
	if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
		return err
	}

	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews/%d", owner, repo, index, id), jsonHeader, nil)
	return err
}

// CreatePullReview create a review to an pull request
func (c *Client) CreatePullReview(owner, repo string, index int64, opt CreatePullReviewOptions) (*PullReview, error) {
	if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}

	r := new(PullReview)
	return r, c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", owner, repo, index),
		jsonHeader, bytes.NewReader(body), r)
}

// SubmitPullReview submit a pending review to an pull request
func (c *Client) SubmitPullReview(owner, repo string, index, id int64, opt SubmitPullReviewOptions) (*PullReview, error) {
	if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}

	r := new(PullReview)
	return r, c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews/%d", owner, repo, index, id),
		jsonHeader, bytes.NewReader(body), r)
}
