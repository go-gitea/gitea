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
	BranchName                    string    `json:"branch_name"`
	EnablePush                    bool      `json:"enable_push"`
	EnablePushWhitelist           bool      `json:"enable_push_whitelist"`
	PushWhitelistUsernames        []string  `json:"push_whitelist_usernames"`
	PushWhitelistTeams            []string  `json:"push_whitelist_teams"`
	PushWhitelistDeployKeys       bool      `json:"push_whitelist_deploy_keys"`
	EnableMergeWhitelist          bool      `json:"enable_merge_whitelist"`
	MergeWhitelistUsernames       []string  `json:"merge_whitelist_usernames"`
	MergeWhitelistTeams           []string  `json:"merge_whitelist_teams"`
	EnableStatusCheck             bool      `json:"enable_status_check"`
	StatusCheckContexts           []string  `json:"status_check_contexts"`
	RequiredApprovals             int64     `json:"required_approvals"`
	EnableApprovalsWhitelist      bool      `json:"enable_approvals_whitelist"`
	ApprovalsWhitelistUsernames   []string  `json:"approvals_whitelist_username"`
	ApprovalsWhitelistTeams       []string  `json:"approvals_whitelist_teams"`
	BlockOnRejectedReviews        bool      `json:"block_on_rejected_reviews"`
	BlockOnOfficialReviewRequests bool      `json:"block_on_official_review_requests"`
	BlockOnOutdatedBranch         bool      `json:"block_on_outdated_branch"`
	DismissStaleApprovals         bool      `json:"dismiss_stale_approvals"`
	RequireSignedCommits          bool      `json:"require_signed_commits"`
	ProtectedFilePatterns         string    `json:"protected_file_patterns"`
	Created                       time.Time `json:"created_at"`
	Updated                       time.Time `json:"updated_at"`
}

// CreateBranchProtectionOption options for creating a branch protection
type CreateBranchProtectionOption struct {
	BranchName                    string   `json:"branch_name"`
	EnablePush                    bool     `json:"enable_push"`
	EnablePushWhitelist           bool     `json:"enable_push_whitelist"`
	PushWhitelistUsernames        []string `json:"push_whitelist_usernames"`
	PushWhitelistTeams            []string `json:"push_whitelist_teams"`
	PushWhitelistDeployKeys       bool     `json:"push_whitelist_deploy_keys"`
	EnableMergeWhitelist          bool     `json:"enable_merge_whitelist"`
	MergeWhitelistUsernames       []string `json:"merge_whitelist_usernames"`
	MergeWhitelistTeams           []string `json:"merge_whitelist_teams"`
	EnableStatusCheck             bool     `json:"enable_status_check"`
	StatusCheckContexts           []string `json:"status_check_contexts"`
	RequiredApprovals             int64    `json:"required_approvals"`
	EnableApprovalsWhitelist      bool     `json:"enable_approvals_whitelist"`
	ApprovalsWhitelistUsernames   []string `json:"approvals_whitelist_username"`
	ApprovalsWhitelistTeams       []string `json:"approvals_whitelist_teams"`
	BlockOnRejectedReviews        bool     `json:"block_on_rejected_reviews"`
	BlockOnOfficialReviewRequests bool     `json:"block_on_official_review_requests"`
	BlockOnOutdatedBranch         bool     `json:"block_on_outdated_branch"`
	DismissStaleApprovals         bool     `json:"dismiss_stale_approvals"`
	RequireSignedCommits          bool     `json:"require_signed_commits"`
	ProtectedFilePatterns         string   `json:"protected_file_patterns"`
}

// EditBranchProtectionOption options for editing a branch protection
type EditBranchProtectionOption struct {
	EnablePush                    *bool    `json:"enable_push"`
	EnablePushWhitelist           *bool    `json:"enable_push_whitelist"`
	PushWhitelistUsernames        []string `json:"push_whitelist_usernames"`
	PushWhitelistTeams            []string `json:"push_whitelist_teams"`
	PushWhitelistDeployKeys       *bool    `json:"push_whitelist_deploy_keys"`
	EnableMergeWhitelist          *bool    `json:"enable_merge_whitelist"`
	MergeWhitelistUsernames       []string `json:"merge_whitelist_usernames"`
	MergeWhitelistTeams           []string `json:"merge_whitelist_teams"`
	EnableStatusCheck             *bool    `json:"enable_status_check"`
	StatusCheckContexts           []string `json:"status_check_contexts"`
	RequiredApprovals             *int64   `json:"required_approvals"`
	EnableApprovalsWhitelist      *bool    `json:"enable_approvals_whitelist"`
	ApprovalsWhitelistUsernames   []string `json:"approvals_whitelist_username"`
	ApprovalsWhitelistTeams       []string `json:"approvals_whitelist_teams"`
	BlockOnRejectedReviews        *bool    `json:"block_on_rejected_reviews"`
	BlockOnOfficialReviewRequests *bool    `json:"block_on_official_review_requests"`
	BlockOnOutdatedBranch         *bool    `json:"block_on_outdated_branch"`
	DismissStaleApprovals         *bool    `json:"dismiss_stale_approvals"`
	RequireSignedCommits          *bool    `json:"require_signed_commits"`
	ProtectedFilePatterns         *string  `json:"protected_file_patterns"`
}

// ListBranchProtectionsOptions list branch protection options
type ListBranchProtectionsOptions struct {
	ListOptions
}

// ListBranchProtections list branch protections for a repo
func (c *Client) ListBranchProtections(owner, repo string, opt ListBranchProtectionsOptions) ([]*BranchProtection, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, nil, err
	}
	bps := make([]*BranchProtection, 0, opt.PageSize)
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/branch_protections", owner, repo))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &bps)
	return bps, resp, err
}

// GetBranchProtection gets a branch protection
func (c *Client) GetBranchProtection(owner, repo, name string) (*BranchProtection, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &name); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, nil, err
	}
	bp := new(BranchProtection)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/branch_protections/%s", owner, repo, name), jsonHeader, nil, bp)
	return bp, resp, err
}

// CreateBranchProtection creates a branch protection for a repo
func (c *Client) CreateBranchProtection(owner, repo string, opt CreateBranchProtectionOption) (*BranchProtection, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, nil, err
	}
	bp := new(BranchProtection)
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/branch_protections", owner, repo), jsonHeader, bytes.NewReader(body), bp)
	return bp, resp, err
}

// EditBranchProtection edits a branch protection for a repo
func (c *Client) EditBranchProtection(owner, repo, name string, opt EditBranchProtectionOption) (*BranchProtection, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &name); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, nil, err
	}
	bp := new(BranchProtection)
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s/branch_protections/%s", owner, repo, name), jsonHeader, bytes.NewReader(body), bp)
	return bp, resp, err
}

// DeleteBranchProtection deletes a branch protection for a repo
func (c *Client) DeleteBranchProtection(owner, repo, name string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &name); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/branch_protections/%s", owner, repo, name), jsonHeader, nil)
	return resp, err
}
