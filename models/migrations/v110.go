// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models"

	"xorm.io/core"
	"xorm.io/xorm"
)

func addBranchProtectionCanPushAndEnableWhitelist(x *xorm.Engine) error {

	type ProtectedBranch struct {
		CanPush                  bool  `xorm:"NOT NULL DEFAULT false"`
		EnableWhitelist          bool  `xorm:"NOT NULL DEFAULT false"`
		EnableApprovalsWhitelist bool  `xorm:"NOT NULL DEFAULT false"`
		RequiredApprovals        int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	type Review struct {
		ID       int64 `xorm:"pk autoincr"`
		Official bool  `xorm:"NOT NULL DEFAULT false"`
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Sync2(new(ProtectedBranch)); err != nil {
		return err
	}

	if err := sess.Sync2(new(Review)); err != nil {
		return err
	}

	if _, err := sess.Exec("UPDATE `protected_branch` SET `can_push` = `enable_whitelist`"); err != nil {
		return err
	}
	if _, err := sess.Exec("UPDATE `protected_branch` SET `enable_approvals_whitelist` = ? WHERE `required_approvals` > ?", true, 0); err != nil {
		return err
	}

	// Find latest review of each user in each pull request, and set official field if appropriate
	reviews := []*models.Review{}
	if x.Dialect().DBType() == core.MSSQL {
		if err := x.SQL(`SELECT *, max(review.updated_unix) as review_updated_unix FROM review WHERE (review.type = ? OR review.type = ?)
GROUP BY review.id, review.issue_id, review.reviewer_id, review.type, ) as review
ORDER BY review_updated_unix DESC`,
			models.ReviewTypeApprove, models.ReviewTypeReject).
			Find(&reviews); err != nil {
			return err
		}
	} else {
		if err := x.Select("review.*, max(review.updated_unix) as review_updated_unix").
			Table("review").
			Join("INNER", "`user`", "review.reviewer_id = `user`.id").
			Where("(review.type = ? OR review.type = ?)",
				models.ReviewTypeApprove, models.ReviewTypeReject).
			GroupBy("review.issue_id, review.reviewer_id, review.type").
			OrderBy("review_updated_unix DESC").
			Find(&reviews); err != nil {
			return err
		}
	}

	// We need to group our results by user id _and_ review type, otherwise the query fails when using postgresql.
	usersInArray := make(map[int64]map[int64]bool)
	for _, review := range reviews {
		if usersInArray[review.IssueID] == nil {
			usersInArray[review.IssueID] = make(map[int64]bool)
		}
		if !usersInArray[review.IssueID][review.ReviewerID] {
			if err := review.LoadAttributes(); err != nil {
				return err
			}
			official, err := models.IsOfficialReviewer(review.Issue, review.Reviewer)
			if err != nil {
				return err
			}
			review.Official = official

			if _, err := sess.ID(review.ID).Cols("official").Update(review); err != nil {
				return err
			}
			usersInArray[review.IssueID][review.ReviewerID] = true
		}
	}

	return sess.Commit()
}
