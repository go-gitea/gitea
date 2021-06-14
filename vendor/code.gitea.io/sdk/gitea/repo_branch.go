// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// PayloadUser represents the author or committer of a commit
type PayloadUser struct {
	// Full name of the commit author
	Name     string `json:"name"`
	Email    string `json:"email"`
	UserName string `json:"username"`
}

// PayloadCommit represents a commit
type PayloadCommit struct {
	// sha1 hash of the commit
	ID           string                     `json:"id"`
	Message      string                     `json:"message"`
	URL          string                     `json:"url"`
	Author       *PayloadUser               `json:"author"`
	Committer    *PayloadUser               `json:"committer"`
	Verification *PayloadCommitVerification `json:"verification"`
	Timestamp    time.Time                  `json:"timestamp"`
	Added        []string                   `json:"added"`
	Removed      []string                   `json:"removed"`
	Modified     []string                   `json:"modified"`
}

// PayloadCommitVerification represents the GPG verification of a commit
type PayloadCommitVerification struct {
	Verified  bool   `json:"verified"`
	Reason    string `json:"reason"`
	Signature string `json:"signature"`
	Payload   string `json:"payload"`
}

// Branch represents a repository branch
type Branch struct {
	Name                          string         `json:"name"`
	Commit                        *PayloadCommit `json:"commit"`
	Protected                     bool           `json:"protected"`
	RequiredApprovals             int64          `json:"required_approvals"`
	EnableStatusCheck             bool           `json:"enable_status_check"`
	StatusCheckContexts           []string       `json:"status_check_contexts"`
	UserCanPush                   bool           `json:"user_can_push"`
	UserCanMerge                  bool           `json:"user_can_merge"`
	EffectiveBranchProtectionName string         `json:"effective_branch_protection_name"`
}

// ListRepoBranchesOptions options for listing a repository's branches
type ListRepoBranchesOptions struct {
	ListOptions
}

// ListRepoBranches list all the branches of one repository
func (c *Client) ListRepoBranches(user, repo string, opt ListRepoBranchesOptions) ([]*Branch, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	branches := make([]*Branch, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/branches?%s", user, repo, opt.getURLQuery().Encode()), nil, nil, &branches)
	return branches, resp, err
}

// GetRepoBranch get one branch's information of one repository
func (c *Client) GetRepoBranch(user, repo, branch string) (*Branch, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &branch); err != nil {
		return nil, nil, err
	}
	b := new(Branch)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/branches/%s", user, repo, branch), nil, nil, &b)
	if err != nil {
		return nil, resp, err
	}
	return b, resp, nil
}

// DeleteRepoBranch delete a branch in a repository
func (c *Client) DeleteRepoBranch(user, repo, branch string) (bool, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &branch); err != nil {
		return false, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return false, nil, err
	}
	status, resp, err := c.getStatusCode("DELETE", fmt.Sprintf("/repos/%s/%s/branches/%s", user, repo, branch), nil, nil)
	if err != nil {
		return false, resp, err
	}
	return status == 204, resp, nil
}

// CreateBranchOption options when creating a branch in a repository
type CreateBranchOption struct {
	// Name of the branch to create
	BranchName string `json:"new_branch_name"`
	// Name of the old branch to create from (optional)
	OldBranchName string `json:"old_branch_name"`
}

// Validate the CreateBranchOption struct
func (opt CreateBranchOption) Validate() error {
	if len(opt.BranchName) == 0 {
		return fmt.Errorf("BranchName is empty")
	}
	if len(opt.BranchName) > 100 {
		return fmt.Errorf("BranchName to long")
	}
	if len(opt.OldBranchName) > 100 {
		return fmt.Errorf("OldBranchName to long")
	}
	return nil
}

// CreateBranch creates a branch for a user's repository
func (c *Client) CreateBranch(owner, repo string, opt CreateBranchOption) (*Branch, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_13_0); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	branch := new(Branch)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/branches", owner, repo), jsonHeader, bytes.NewReader(body), branch)
	return branch, resp, err
}
