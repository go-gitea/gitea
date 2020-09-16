// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models"

	"xorm.io/xorm"
)

func addBranchProtectionCanPushAndEnableWhitelist(x *xorm.Engine) error {
	type ProtectedBranch struct {
		CanPush                  bool  `xorm:"NOT NULL DEFAULT false"`
		EnableApprovalsWhitelist bool  `xorm:"NOT NULL DEFAULT false"`
		RequiredApprovals        int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	type Review struct {
		ID       int64 `xorm:"pk autoincr"`
		Official bool  `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync2(new(ProtectedBranch)); err != nil {
		return err
	}

	if err := x.Sync2(new(Review)); err != nil {
		return err
	}

	if _, err := x.Exec("UPDATE `protected_branch` SET `enable_whitelist` = ? WHERE enable_whitelist IS NULL", false); err != nil {
		return err
	}
	if _, err := x.Exec("UPDATE `protected_branch` SET `can_push` = `enable_whitelist`"); err != nil {
		return err
	}
	if _, err := x.Exec("UPDATE `protected_branch` SET `enable_approvals_whitelist` = ? WHERE `required_approvals` > ?", true, 0); err != nil {
		return err
	}

	var pageSize int64 = 20
	qresult, err := x.QueryInterface("SELECT max(id) as max_id FROM issue")
	if err != nil {
		return err
	}
	var totalIssues int64
	totalIssues, ok := qresult[0]["max_id"].(int64)
	if !ok {
		// If there are no issues at all we ignore it
		return nil
	}
	totalPages := totalIssues / pageSize

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	// Find latest review of each user in each pull request, and set official field if appropriate

	var page int64
	var count int
	for page = 0; page <= totalPages; page++ {
		reviews := []*models.Review{}
		if err := sess.SQL("SELECT * FROM review WHERE id IN (SELECT max(id) as id FROM review WHERE issue_id > ? AND issue_id <= ? AND type in (?, ?) GROUP BY issue_id, reviewer_id)",
			page*pageSize, (page+1)*pageSize, models.ReviewTypeApprove, models.ReviewTypeReject).
			Find(&reviews); err != nil {
			return err
		}

		for _, review := range reviews {
			if err := review.LoadAttributesX(sess); err != nil {
				// Error might occur if user or issue doesn't exist, ignore it.
				continue
			}
			official, err := models.IsOfficialReviewerX(sess, review.Issue, review.Reviewer)
			if err != nil {
				// Branch might not be proteced or other error, ignore it.
				continue
			}
			review.Official = official

			count++

			if _, err := sess.ID(review.ID).Cols("official").Update(review); err != nil {
				return err
			}

			if count == 100 {
				if err := sess.Commit(); err != nil {
					return err
				}
				count = 0
				if err := sess.Begin(); err != nil {
					return err
				}
			}
		}
	}

	if count > 0 {
		return sess.Commit()
	}
	return nil
}
