// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
)

func init() {
	Register(&Check{
		Title:                      "Fix attachment which have issue_id or release_id lost repo_id",
		Name:                       "fix-attachment-repo-id",
		IsDefault:                  false,
		Run:                        fixAttachmentRepoIDCheck,
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})
}

func fixAttachmentRepoIDCheck(ctx context.Context, logger log.Logger, autofix bool) error {
	countIssue, err := db.GetEngine(ctx).
		Where("`issue_id` > 0 AND (`repo_id` IS NULL OR `repo_id` = 0)").
		Table("attachment").Cols("id").Count()
	if err != nil {
		return err
	}

	countRelease, err := db.GetEngine(ctx).
		Where("`release_id` > 0 AND (`repo_id` IS NULL OR `repo_id` = 0)").
		Table("attachment").Cols("id").Count()
	if err != nil {
		return err
	}
	count := countIssue + countRelease
	if count == 0 {
		logger.Info("No attachment repo_id issues found.")
		return nil
	}

	logger.Warn("Found %d(issue), %d(release) attachments with missing repo_id.", countIssue, countRelease)

	if !autofix {
		return nil
	}

	updatedIssue, err := db.GetEngine(ctx).Exec("UPDATE `attachment` SET `repo_id` = (SELECT `repo_id` FROM `issue` WHERE `issue`.`id` = `attachment`.`issue_id`) WHERE `issue_id` > 0 AND (`repo_id` IS NULL OR `repo_id` = 0);")
	if err != nil {
		return err
	}
	cntIssue, _ := updatedIssue.RowsAffected()

	updatedRelease, err := db.GetEngine(ctx).Exec("UPDATE `attachment` SET `repo_id` = (SELECT `repo_id` FROM `release` WHERE `release`.`id` = `attachment`.`release_id`) WHERE `release_id` > 0 AND (`repo_id` IS NULL OR `repo_id` = 0);")
	if err != nil {
		return err
	}
	cntRelease, _ := updatedRelease.RowsAffected()

	logger.Info("Fixed attachment repo_id issues %d and release %d successfully.", cntIssue, cntRelease)

	return nil
}
