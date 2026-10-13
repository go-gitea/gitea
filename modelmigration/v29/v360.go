// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/json"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/xorm"
)

type protectedBranchConfigV358 struct {
	CanPush                       bool     `xorm:"NOT NULL DEFAULT false" json:"can_push,omitzero"`
	EnableWhitelist               bool     `json:"enable_whitelist,omitzero"`
	WhitelistUserIDs              []int64  `xorm:"JSON TEXT" json:"whitelist_user_ids,omitzero"`
	WhitelistTeamIDs              []int64  `xorm:"JSON TEXT" json:"whitelist_team_ids,omitzero"`
	EnableMergeWhitelist          bool     `xorm:"NOT NULL DEFAULT false" json:"enable_merge_whitelist,omitzero"`
	WhitelistDeployKeys           bool     `xorm:"NOT NULL DEFAULT false" json:"-"`
	MergeWhitelistUserIDs         []int64  `xorm:"JSON TEXT" json:"merge_whitelist_user_ids,omitzero"`
	MergeWhitelistTeamIDs         []int64  `xorm:"JSON TEXT" json:"merge_whitelist_team_ids,omitzero"`
	EnableBypassAllowlist         bool     `xorm:"NOT NULL DEFAULT false" json:"enable_bypass_allowlist,omitzero"`
	BypassAllowlistUserIDs        []int64  `xorm:"JSON TEXT" json:"bypass_allowlist_user_ids,omitzero"`
	BypassAllowlistTeamIDs        []int64  `xorm:"JSON TEXT" json:"bypass_allowlist_team_ids,omitzero"`
	CanForcePush                  bool     `xorm:"NOT NULL DEFAULT false" json:"can_force_push,omitzero"`
	EnableForcePushAllowlist      bool     `xorm:"NOT NULL DEFAULT false" json:"enable_force_push_allowlist,omitzero"`
	ForcePushAllowlistUserIDs     []int64  `xorm:"JSON TEXT" json:"force_push_allowlist_user_ids,omitzero"`
	ForcePushAllowlistTeamIDs     []int64  `xorm:"JSON TEXT" json:"force_push_allowlist_team_ids,omitzero"`
	ForcePushAllowlistDeployKeys  bool     `xorm:"NOT NULL DEFAULT false" json:"-"`
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

type intermediateProtectedBranch struct {
	ID        int64                     `xorm:"pk autoincr"`
	NewConfig string                    `xorm:"LONGTEXT config"`
	OldConfig protectedBranchConfigV358 `xorm:"extends"`
}

func (intermediateProtectedBranch) TableName() string { return "protected_branch" }

func MoveBranchProtectionConfigToJSON(ctx context.Context, x base.EngineMigration) error {
	if _, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(protectedBranchV358)); err != nil {
		return err
	}
	exists, err := x.Dialect().IsColumnExist(x.DB(), ctx, "protected_branch", "can_push")
	if err != nil || !exists {
		return err
	}

	err = base.EngineTransaction(x, func(sess base.Session) error {
		err := base.IterateTableByColumn(ctx, x, sess, "id", nil, func(ctx context.Context, pb *intermediateProtectedBranch) error {
			if pb.NewConfig != "" {
				return nil // skip the already migrated fields
			}
			const deployKeyUserID int64 = -3 // frozen user.DeployKeyUserID
			if pb.OldConfig.WhitelistDeployKeys {
				pb.OldConfig.WhitelistUserIDs = append(pb.OldConfig.WhitelistUserIDs, deployKeyUserID)
			}
			if pb.OldConfig.ForcePushAllowlistDeployKeys {
				pb.OldConfig.ForcePushAllowlistUserIDs = append(pb.OldConfig.ForcePushAllowlistUserIDs, deployKeyUserID)
			}
			configJson, err := json.Marshal(pb.OldConfig)
			util.MustNoError(err)
			_, err = sess.ID(pb.ID).Cols("config").Update(&protectedBranchV358{Config: string(configJson)})
			return err
		})
		if err != nil {
			return err
		}
		legacyTableInfo, err := x.TableInfo(new(protectedBranchConfigV358))
		if err != nil {
			return err
		}
		return base.DropTableColumns(sess, "protected_branch", legacyTableInfo.ColumnsSeq()...)
	})
	if err != nil {
		return err
	}
	// SQLite rebuilds the table when dropping columns, so restore its unique index.
	return x.Sync(new(protectedBranchV358))
}
