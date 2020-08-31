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

// BranchProtection represents a branch protection for a repository
type BranchProtection struct {
	BranchName                  string   `json:"branch_name"`
	EnablePush                  bool     `json:"enable_push"`
	EnablePushWhitelist         bool     `json:"enable_push_whitelist"`
	PushWhitelistUsernames      []string `json:"push_whitelist_usernames"`
	PushWhitelistTeams          []string `json:"push_whitelist_teams"`
	PushWhitelistDeployKeys     bool     `json:"push_whitelist_deploy_keys"`
	EnableMergeWhitelist        bool     `json:"enable_merge_whitelist"`
	MergeWhitelistUsernames     []string `json:"merge_whitelist_usernames"`
	MergeWhitelistTeams         []string `json:"merge_whitelist_teams"`
	EnableStatusCheck           bool     `json:"enable_status_check"`
	StatusCheckContexts         []string `json:"status_check_contexts"`
	RequiredApprovals           int64    `json:"required_approvals"`
	EnableApprovalsWhitelist    bool     `json:"enable_approvals_whitelist"`
	ApprovalsWhitelistUsernames []string `json:"approvals_whitelist_username"`
	ApprovalsWhitelistTeams     []string `json:"approvals_whitelist_teams"`
	BlockOnRejectedReviews      bool     `json:"block_on_rejected_reviews"`
	BlockOnOutdatedBranch       bool     `json:"block_on_outdated_branch"`
	DismissStaleApprovals       bool     `json:"dismiss_stale_approvals"`
	RequireSignedCommits        bool     `json:"require_signed_commits"`
	ProtectedFilePatterns       string   `json:"protected_file_patterns"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
}

// CreateBranchProtectionOption options for creating a branch protection
type CreateBranchProtectionOption struct {
	BranchName                  string   `json:"branch_name"`
	EnablePush                  bool     `json:"enable_push"`
	EnablePushWhitelist         bool     `json:"enable_push_whitelist"`
	PushWhitelistUsernames      []string `json:"push_whitelist_usernames"`
	PushWhitelistTeams          []string `json:"push_whitelist_teams"`
	PushWhitelistDeployKeys     bool     `json:"push_whitelist_deploy_keys"`
	EnableMergeWhitelist        bool     `json:"enable_merge_whitelist"`
	MergeWhitelistUsernames     []string `json:"merge_whitelist_usernames"`
	MergeWhitelistTeams         []string `json:"merge_whitelist_teams"`
	EnableStatusCheck           bool     `json:"enable_status_check"`
	StatusCheckContexts         []string `json:"status_check_contexts"`
	RequiredApprovals           int64    `json:"required_approvals"`
	EnableApprovalsWhitelist    bool     `json:"enable_approvals_whitelist"`
	ApprovalsWhitelistUsernames []string `json:"approvals_whitelist_username"`
	ApprovalsWhitelistTeams     []string `json:"approvals_whitelist_teams"`
	BlockOnRejectedReviews      bool     `json:"block_on_rejected_reviews"`
	BlockOnOutdatedBranch       bool     `json:"block_on_outdated_branch"`
	DismissStaleApprovals       bool     `json:"dismiss_stale_approvals"`
	RequireSignedCommits        bool     `json:"require_signed_commits"`
	ProtectedFilePatterns       string   `json:"protected_file_patterns"`
}

// EditBranchProtectionOption options for editing a branch protection
type EditBranchProtectionOption struct {
	EnablePush                  *bool    `json:"enable_push"`
	EnablePushWhitelist         *bool    `json:"enable_push_whitelist"`
	PushWhitelistUsernames      []string `json:"push_whitelist_usernames"`
	PushWhitelistTeams          []string `json:"push_whitelist_teams"`
	PushWhitelistDeployKeys     *bool    `json:"push_whitelist_deploy_keys"`
	EnableMergeWhitelist        *bool    `json:"enable_merge_whitelist"`
	MergeWhitelistUsernames     []string `json:"merge_whitelist_usernames"`
	MergeWhitelistTeams         []string `json:"merge_whitelist_teams"`
	EnableStatusCheck           *bool    `json:"enable_status_check"`
	StatusCheckContexts         []string `json:"status_check_contexts"`
	RequiredApprovals           *int64   `json:"required_approvals"`
	EnableApprovalsWhitelist    *bool    `json:"enable_approvals_whitelist"`
	ApprovalsWhitelistUsernames []string `json:"approvals_whitelist_username"`
	ApprovalsWhitelistTeams     []string `json:"approvals_whitelist_teams"`
	BlockOnRejectedReviews      *bool    `json:"block_on_rejected_reviews"`
	BlockOnOutdatedBranch       *bool    `json:"block_on_outdated_branch"`
	DismissStaleApprovals       *bool    `json:"dismiss_stale_approvals"`
	RequireSignedCommits        *bool    `json:"require_signed_commits"`
	ProtectedFilePatterns       *string  `json:"protected_file_patterns"`
}

// ListBranchProtectionsOptions list branch protection options
type ListBranchProtectionsOptions struct {
	ListOptions
}

// ListBranchProtections list branch protections for a repo
func (c *Client) ListBranchProtections(owner, repo string, opt ListBranchProtectionsOptions) ([]*BranchProtection, error) {
	if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
		return nil, err
	}
	bps := make([]*BranchProtection, 0, 5)
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/branch_protections", owner, repo))
	link.RawQuery = opt.getURLQuery().Encode()
	return bps, c.getParsedResponse("GET", link.String(), jsonHeader, nil, &bps)
}

// GetBranchProtection gets a branch protection
func (c *Client) GetBranchProtection(owner, repo, name string) (*BranchProtection, error) {
	if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
		return nil, err
	}
	bp := new(BranchProtection)
	return bp, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/branch_protections/%s", owner, repo, name), jsonHeader, nil, bp)
}

// CreateBranchProtection creates a branch protection for a repo
func (c *Client) CreateBranchProtection(owner, repo string, opt CreateBranchProtectionOption) (*BranchProtection, error) {
	if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
		return nil, err
	}
	bp := new(BranchProtection)
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return bp, c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/branch_protections", owner, repo), jsonHeader, bytes.NewReader(body), bp)
}

// EditBranchProtection edits a branch protection for a repo
func (c *Client) EditBranchProtection(owner, repo, name string, opt EditBranchProtectionOption) (*BranchProtection, error) {
	if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
		return nil, err
	}
	bp := new(BranchProtection)
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return bp, c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s/branch_protections/%s", owner, repo, name), jsonHeader, bytes.NewReader(body), bp)
}

// DeleteBranchProtection deletes a branch protection for a repo
func (c *Client) DeleteBranchProtection(owner, repo, name string) error {
	if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
		return err
	}
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/branch_protections/%s", owner, repo, name), jsonHeader, nil)
	return err
}
