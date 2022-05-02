// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/notification"
)

func changeMilestoneAssign(ctx context.Context, doer *user_model.User, issue *models.Issue, oldMilestoneID int64) error {
	if err := models.UpdateIssueCols(ctx, issue, "milestone_id"); err != nil {
		return err
	}

	if oldMilestoneID > 0 {
		if err := issues_model.UpdateMilestoneCounters(ctx, oldMilestoneID); err != nil {
			return err
		}
	}

	if issue.MilestoneID > 0 {
		if err := issues_model.UpdateMilestoneCounters(ctx, issue.MilestoneID); err != nil {
			return err
		}
	}

	if oldMilestoneID > 0 || issue.MilestoneID > 0 {
		if err := issue.LoadRepo(ctx); err != nil {
			return err
		}

		opts := &models.CreateCommentOptions{
			Type:           models.CommentTypeMilestone,
			Doer:           doer,
			Repo:           issue.Repo,
			Issue:          issue,
			OldMilestoneID: oldMilestoneID,
			MilestoneID:    issue.MilestoneID,
		}
		if _, err := models.CreateCommentCtx(ctx, opts); err != nil {
			return err
		}
	}

	return nil
}

// ChangeMilestoneAssign changes assignment of milestone for issue.
func ChangeMilestoneAssign(issue *models.Issue, doer *user_model.User, oldMilestoneID int64) (err error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = changeMilestoneAssign(ctx, doer, issue, oldMilestoneID); err != nil {
		return err
	}

	if err = committer.Commit(); err != nil {
		return fmt.Errorf("Commit: %v", err)
	}

	notification.NotifyIssueChangeMilestone(doer, issue, oldMilestoneID)

	return nil
}
