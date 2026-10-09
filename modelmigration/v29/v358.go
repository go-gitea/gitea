// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/json"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm"
)

type protectedBranchConfigV358 struct {
	CanPush                       bool     `xorm:"NOT NULL DEFAULT false" json:"can_push,omitzero"`
	EnableWhitelist               bool     `json:"enable_whitelist,omitzero"`
	WhitelistUserIDs              []int64  `xorm:"JSON TEXT" json:"whitelist_user_ids,omitzero"`
	WhitelistTeamIDs              []int64  `xorm:"JSON TEXT" json:"whitelist_team_ids,omitzero"`
	EnableMergeWhitelist          bool     `xorm:"NOT NULL DEFAULT false" json:"enable_merge_whitelist,omitzero"`
	WhitelistDeployKeys           bool     `xorm:"NOT NULL DEFAULT false" json:"whitelist_deploy_keys,omitzero"`
	MergeWhitelistUserIDs         []int64  `xorm:"JSON TEXT" json:"merge_whitelist_user_ids,omitzero"`
	MergeWhitelistTeamIDs         []int64  `xorm:"JSON TEXT" json:"merge_whitelist_team_ids,omitzero"`
	EnableBypassAllowlist         bool     `xorm:"NOT NULL DEFAULT false" json:"enable_bypass_allowlist,omitzero"`
	BypassAllowlistUserIDs        []int64  `xorm:"JSON TEXT" json:"bypass_allowlist_user_ids,omitzero"`
	BypassAllowlistTeamIDs        []int64  `xorm:"JSON TEXT" json:"bypass_allowlist_team_ids,omitzero"`
	CanForcePush                  bool     `xorm:"NOT NULL DEFAULT false" json:"can_force_push,omitzero"`
	EnableForcePushAllowlist      bool     `xorm:"NOT NULL DEFAULT false" json:"enable_force_push_allowlist,omitzero"`
	ForcePushAllowlistUserIDs     []int64  `xorm:"JSON TEXT" json:"force_push_allowlist_user_ids,omitzero"`
	ForcePushAllowlistTeamIDs     []int64  `xorm:"JSON TEXT" json:"force_push_allowlist_team_ids,omitzero"`
	ForcePushAllowlistDeployKeys  bool     `xorm:"NOT NULL DEFAULT false" json:"force_push_allowlist_deploy_keys,omitzero"`
	EnableStatusCheck             bool     `xorm:"NOT NULL DEFAULT false" json:"enable_status_check,omitzero"`
	StatusCheckContexts           []string `xorm:"JSON TEXT" json:"status_check_contexts,omitzero"`
	EnableApprovalsWhitelist      bool     `xorm:"NOT NULL DEFAULT false" json:"enable_approvals_whitelist,omitzero"`
	ApprovalsWhitelistUserIDs     []int64  `xorm:"JSON TEXT" json:"approvals_whitelist_user_ids,omitzero"`
	ApprovalsWhitelistTeamIDs     []int64  `xorm:"JSON TEXT" json:"approvals_whitelist_team_ids,omitzero"`
	RequiredApprovals             int64    `xorm:"NOT NULL DEFAULT 0" json:"required_approvals,omitzero"`
	BlockOnRejectedReviews        bool     `xorm:"NOT NULL DEFAULT false" json:"block_on_rejected_reviews,omitzero"`
	BlockOnOfficialReviewRequests bool     `xorm:"NOT NULL DEFAULT false" json:"block_on_official_review_requests,omitzero"`
	BlockOnCodeownerReviews       bool     `xorm:"NOT NULL DEFAULT false" json:"block_on_codeowner_reviews,omitzero"`
	BlockOnOutdatedBranch         bool     `xorm:"NOT NULL DEFAULT false" json:"block_on_outdated_branch,omitzero"`
	DismissStaleApprovals         bool     `xorm:"NOT NULL DEFAULT false" json:"dismiss_stale_approvals,omitzero"`
	IgnoreStaleApprovals          bool     `xorm:"NOT NULL DEFAULT false" json:"ignore_stale_approvals,omitzero"`
	RequireSignedCommits          bool     `xorm:"NOT NULL DEFAULT false" json:"require_signed_commits,omitzero"`
	ProtectedFilePatterns         string   `xorm:"TEXT" json:"protected_file_patterns,omitzero"`
	UnprotectedFilePatterns       string   `xorm:"TEXT" json:"unprotected_file_patterns,omitzero"`
	BlockAdminMergeOverride       bool     `xorm:"NOT NULL DEFAULT false" json:"block_admin_merge_override,omitzero"`
}

type protectedBranchV358 struct {
	ID          int64  `xorm:"pk autoincr"`
	RepoID      int64  `xorm:"UNIQUE(s)"`
	BranchName  string `xorm:"UNIQUE(s)"`
	Priority    int64  `xorm:"NOT NULL DEFAULT 0"`
	Config      string `xorm:"LONGTEXT"`
	CreatedUnix timeutil.TimeStamp
	UpdatedUnix timeutil.TimeStamp
}

func (protectedBranchV358) TableName() string { return "protected_branch" }

func MoveBranchProtectionConfigToJSON(ctx context.Context, x base.EngineMigration) error {
	if _, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(protectedBranchV358)); err != nil {
		return err
	}
	exists, err := x.Dialect().IsColumnExist(x.DB(), ctx, "protected_branch", "can_push")
	if err != nil || !exists {
		return err
	}

	type legacyProtectedBranch struct {
		ID     int64
		Config protectedBranchConfigV358 `xorm:"extends"`
	}
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	for lastID := int64(0); ; {
		var branches []legacyProtectedBranch
		if err := sess.Table("protected_branch").Where("id > ?", lastID).Asc("id").Limit(100).Find(&branches); err != nil {
			return err
		}
		if len(branches) == 0 {
			break
		}
		for _, branch := range branches {
			config, err := json.Marshal(branch.Config)
			if err != nil {
				return err
			}
			if _, err := sess.ID(branch.ID).Cols("config").Update(&protectedBranchV358{Config: string(config)}); err != nil {
				return err
			}
			lastID = branch.ID
		}
	}
	legacyTable, err := x.TableInfo(new(protectedBranchConfigV358))
	if err != nil {
		return err
	}
	if err := base.DropTableColumns(sess, "protected_branch", legacyTable.ColumnsSeq()...); err != nil {
		return err
	}
	if err := sess.Commit(); err != nil {
		return err
	}
	// SQLite rebuilds the table when dropping columns, so restore its unique index.
	return x.Sync(new(protectedBranchV358))
}
