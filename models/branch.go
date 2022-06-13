// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/modules/log"
)

// HasEnoughApprovals returns true if pr has enough granted approvals.
func HasEnoughApprovals(ctx context.Context, protectBranch *git_model.ProtectedBranch, pr *PullRequest) bool {
	if protectBranch.RequiredApprovals == 0 {
		return true
	}
	return GetGrantedApprovalsCount(ctx, protectBranch, pr) >= protectBranch.RequiredApprovals
}

// GetGrantedApprovalsCount returns the number of granted approvals for pr. A granted approval must be authored by a user in an approval whitelist.
func GetGrantedApprovalsCount(ctx context.Context, protectBranch *git_model.ProtectedBranch, pr *PullRequest) int64 {
	sess := db.GetEngine(ctx).Where("issue_id = ?", pr.IssueID).
		And("type = ?", ReviewTypeApprove).
		And("official = ?", true).
		And("dismissed = ?", false)
	if protectBranch.DismissStaleApprovals {
		sess = sess.And("stale = ?", false)
	}
	approvals, err := sess.Count(new(Review))
	if err != nil {
		log.Error("GetGrantedApprovalsCount: %v", err)
		return 0
	}

	return approvals
}

// MergeBlockedByRejectedReview returns true if merge is blocked by rejected reviews
func MergeBlockedByRejectedReview(ctx context.Context, protectBranch *git_model.ProtectedBranch, pr *PullRequest) bool {
	if !protectBranch.BlockOnRejectedReviews {
		return false
	}
	rejectExist, err := db.GetEngine(ctx).Where("issue_id = ?", pr.IssueID).
		And("type = ?", ReviewTypeReject).
		And("official = ?", true).
		And("dismissed = ?", false).
		Exist(new(Review))
	if err != nil {
		log.Error("MergeBlockedByRejectedReview: %v", err)
		return true
	}

	return rejectExist
}

// MergeBlockedByOfficialReviewRequests block merge because of some review request to official reviewer
// of from official review
func MergeBlockedByOfficialReviewRequests(ctx context.Context, protectBranch *git_model.ProtectedBranch, pr *PullRequest) bool {
	if !protectBranch.BlockOnOfficialReviewRequests {
		return false
	}
	has, err := db.GetEngine(ctx).Where("issue_id = ?", pr.IssueID).
		And("type = ?", ReviewTypeRequest).
		And("official = ?", true).
		Exist(new(Review))
	if err != nil {
		log.Error("MergeBlockedByOfficialReviewRequests: %v", err)
		return true
	}

	return has
}

// MergeBlockedByOutdatedBranch returns true if merge is blocked by an outdated head branch
func MergeBlockedByOutdatedBranch(protectBranch *git_model.ProtectedBranch, pr *PullRequest) bool {
	return protectBranch.BlockOnOutdatedBranch && pr.CommitsBehind > 0
}
