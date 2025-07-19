// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
)

// checkCommitSHAOfPullRequestCodeReviewComment will check if the commit SHA of pull request code review comments
// For a comment with negative line number, it should be the merge base of the pull request if the comment is on the files page
// if it's on a special commit page or a range commit page, it should be the previous commit when reviewing that commit/commit range
// so that this may be broken for those comments submitted in a special commit(non the first one) page or a range commit page
// NOTICE: the fix can only be done once, so it should be run twice or more
func checkCommitSHAOfPullRequestCodeReviewComment(ctx context.Context, logger log.Logger, autofix bool) error {
	count, err := db.GetEngine(ctx).SQL("SELECT 1 FROM comment where line < 0 AND commit_sha != (select merge_base from pull_request WHERE issue_id = comment.issue_id)").Count()
	if err != nil {
		logger.Critical("Error: %v whilst counting wrong comment commit sha", err)
		return err
	}
	if count > 0 {
		if autofix {
			total, err := db.GetEngine(ctx).Exec("UPDATE comment SET commit_sha = (select merge_base from pull_request WHERE issue_id = comment.issue_id) WHERE line < 0")
			if err != nil {
				return err
			}
			logger.Info("%d comments with wrong commit sha fixed\nWARNING: This doctor can only fix this once, so it should NOT be run twice or more", total)
		} else {
			logger.Warn("%d comments with wrong commit sha exist", count)
		}
	}
	return nil
}

func init() {
	Register(&Check{
		Title:     "Check if comment with negative line number has wrong commit sha",
		Name:      "check-commitsha-review-comment",
		IsDefault: true,
		Run:       checkCommitSHAOfPullRequestCodeReviewComment,
		Priority:  3,
	})
}
