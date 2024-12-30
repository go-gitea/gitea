// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	notify_service "code.gitea.io/gitea/services/notify"
)

func changeMilestoneAssign(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldMilestoneID int64) error {
	// Only check if milestone exists if we don't remove it.
	if issue.MilestoneID > 0 {
		has, err := issues_model.HasMilestoneByRepoID(ctx, issue.RepoID, issue.MilestoneID)
		if err != nil {
			return fmt.Errorf("HasMilestoneByRepoID: %w", err)
		}
		if !has {
			return fmt.Errorf("HasMilestoneByRepoID: issue doesn't exist")
		}
	}

	if err := issues_model.UpdateIssueCols(ctx, issue, "milestone_id"); err != nil {
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

		opts := &issues_model.CreateCommentOptions{
			Type:           issues_model.CommentTypeMilestone,
			Doer:           doer,
			Repo:           issue.Repo,
			Issue:          issue,
			OldMilestoneID: oldMilestoneID,
			MilestoneID:    issue.MilestoneID,
		}
		if _, err := issues_model.CreateComment(ctx, opts); err != nil {
			return err
		}
	}

	if issue.MilestoneID == 0 {
		issue.Milestone = nil
	}

	return nil
}

// ChangeMilestoneAssign changes assignment of milestone for issue.
func ChangeMilestoneAssign(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, oldMilestoneID int64) (err error) {
	dbCtx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = changeMilestoneAssign(dbCtx, doer, issue, oldMilestoneID); err != nil {
		return err
	}

	if err = committer.Commit(); err != nil {
		return fmt.Errorf("Commit: %w", err)
	}

	notify_service.IssueChangeMilestone(ctx, doer, issue, oldMilestoneID)

	return nil
}
