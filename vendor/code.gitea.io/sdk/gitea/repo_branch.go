// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
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

// FIXME: consider using same format as API when commits API are added.
//        applies to PayloadCommit and PayloadCommitVerification

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
func (c *Client) ListRepoBranches(user, repo string, opt ListRepoBranchesOptions) ([]*Branch, error) {
	opt.setDefaults()
	branches := make([]*Branch, 0, opt.PageSize)
	return branches, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/branches?%s", user, repo, opt.getURLQuery().Encode()), nil, nil, &branches)
}

// GetRepoBranch get one branch's information of one repository
func (c *Client) GetRepoBranch(user, repo, branch string) (*Branch, error) {
	b := new(Branch)
	if err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/branches/%s", user, repo, branch), nil, nil, &b); err != nil {
		return nil, err
	}
	return b, nil
}

// DeleteRepoBranch delete a branch in a repository
func (c *Client) DeleteRepoBranch(user, repo, branch string) (bool, error) {
	if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
		return false, err
	}
	status, err := c.getStatusCode("DELETE", fmt.Sprintf("/repos/%s/%s/branches/%s", user, repo, branch), nil, nil)
	if err != nil {
		return false, err
	}
	return status == 204, nil
}
