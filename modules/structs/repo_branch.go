// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

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

// BranchProtection represents a branch protection for a repository
type BranchProtection struct {
	// Deprecated: true
	BranchName                    string   `json:"branch_name"`
	RuleName                      string   `json:"rule_name"`
	Priority                      int64    `json:"priority"`
	EnablePush                    bool     `json:"enable_push"`
	EnablePushWhitelist           bool     `json:"enable_push_whitelist"`
	PushWhitelistUsernames        []string `json:"push_whitelist_usernames"`
	PushWhitelistTeams            []string `json:"push_whitelist_teams"`
	PushWhitelistDeployKeys       bool     `json:"push_whitelist_deploy_keys"`
	EnableForcePush               bool     `json:"enable_force_push"`
	EnableForcePushAllowlist      bool     `json:"enable_force_push_allowlist"`
	ForcePushAllowlistUsernames   []string `json:"force_push_allowlist_usernames"`
	ForcePushAllowlistTeams       []string `json:"force_push_allowlist_teams"`
	ForcePushAllowlistDeployKeys  bool     `json:"force_push_allowlist_deploy_keys"`
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
	IgnoreStaleApprovals          bool     `json:"ignore_stale_approvals"`
	RequireSignedCommits          bool     `json:"require_signed_commits"`
	ProtectedFilePatterns         string   `json:"protected_file_patterns"`
	UnprotectedFilePatterns       string   `json:"unprotected_file_patterns"`
	BlockAdminMergeOverride       bool     `json:"block_admin_merge_override"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
}

// CreateBranchProtectionOption options for creating a branch protection
type CreateBranchProtectionOption struct {
	// Deprecated: true
	BranchName                    string   `json:"branch_name"`
	RuleName                      string   `json:"rule_name"`
	Priority                      int64    `json:"priority"`
	EnablePush                    bool     `json:"enable_push"`
	EnablePushWhitelist           bool     `json:"enable_push_whitelist"`
	PushWhitelistUsernames        []string `json:"push_whitelist_usernames"`
	PushWhitelistTeams            []string `json:"push_whitelist_teams"`
	PushWhitelistDeployKeys       bool     `json:"push_whitelist_deploy_keys"`
	EnableForcePush               bool     `json:"enable_force_push"`
	EnableForcePushAllowlist      bool     `json:"enable_force_push_allowlist"`
	ForcePushAllowlistUsernames   []string `json:"force_push_allowlist_usernames"`
	ForcePushAllowlistTeams       []string `json:"force_push_allowlist_teams"`
	ForcePushAllowlistDeployKeys  bool     `json:"force_push_allowlist_deploy_keys"`
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
	IgnoreStaleApprovals          bool     `json:"ignore_stale_approvals"`
	RequireSignedCommits          bool     `json:"require_signed_commits"`
	ProtectedFilePatterns         string   `json:"protected_file_patterns"`
	UnprotectedFilePatterns       string   `json:"unprotected_file_patterns"`
	BlockAdminMergeOverride       bool     `json:"block_admin_merge_override"`
}

// EditBranchProtectionOption options for editing a branch protection
type EditBranchProtectionOption struct {
	Priority                      *int64   `json:"priority"`
	EnablePush                    *bool    `json:"enable_push"`
	EnablePushWhitelist           *bool    `json:"enable_push_whitelist"`
	PushWhitelistUsernames        []string `json:"push_whitelist_usernames"`
	PushWhitelistTeams            []string `json:"push_whitelist_teams"`
	PushWhitelistDeployKeys       *bool    `json:"push_whitelist_deploy_keys"`
	EnableForcePush               *bool    `json:"enable_force_push"`
	EnableForcePushAllowlist      *bool    `json:"enable_force_push_allowlist"`
	ForcePushAllowlistUsernames   []string `json:"force_push_allowlist_usernames"`
	ForcePushAllowlistTeams       []string `json:"force_push_allowlist_teams"`
	ForcePushAllowlistDeployKeys  *bool    `json:"force_push_allowlist_deploy_keys"`
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
	IgnoreStaleApprovals          *bool    `json:"ignore_stale_approvals"`
	RequireSignedCommits          *bool    `json:"require_signed_commits"`
	ProtectedFilePatterns         *string  `json:"protected_file_patterns"`
	UnprotectedFilePatterns       *string  `json:"unprotected_file_patterns"`
	BlockAdminMergeOverride       *bool    `json:"block_admin_merge_override"`
}

// UpdateBranchProtectionPriories a list to update the branch protection rule priorities
type UpdateBranchProtectionPriories struct {
	IDs []int64 `json:"ids"`
}
