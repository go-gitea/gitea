// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package v1_22 //nolint

import (
	"errors"
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func expandHashReferencesToSha256(x *xorm.Engine) error {
	alteredTables := [][2]string{
		{"commit_status", "context_hash"},
		{"comment", "commit_sha"},
		{"pull_request", "merge_base"},
		{"pull_request", "merged_commit_id"},
		{"review", "commit_id"},
		{"review_state", "commit_sha"},
		{"repo_archiver", "commit_id"},
		{"release", "sha1"},
		{"repo_indexer_status", "commit_sha"},
	}

	db := x.NewSession()
	defer db.Close()

	if err := db.Begin(); err != nil {
		return err
	}

	if !setting.Database.Type.IsSQLite3() {
		if setting.Database.Type.IsMSSQL() {
			// drop indexes that need to be re-created afterwards
			droppedIndexes := []string{
				"DROP INDEX IF EXISTS [IDX_commit_status_context_hash] ON [commit_status]",
				"DROP INDEX IF EXISTS [UQE_review_state_pull_commit_user] ON [review_state]",
				"DROP INDEX IF EXISTS [UQE_repo_archiver_s] ON [repo_archiver]",
			}
			for _, s := range droppedIndexes {
				_, err := db.Exec(s)
				if err != nil {
					return errors.New(s + " " + err.Error())
				}
			}
		}

		for _, alts := range alteredTables {
			var err error
			if setting.Database.Type.IsMySQL() {
				_, err = db.Exec(fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `%s` VARCHAR(64)", alts[0], alts[1]))
			} else if setting.Database.Type.IsMSSQL() {
				_, err = db.Exec(fmt.Sprintf("ALTER TABLE [%s] ALTER COLUMN [%s] NVARCHAR(64)", alts[0], alts[1]))
			} else {
				_, err = db.Exec(fmt.Sprintf("ALTER TABLE `%s` ALTER COLUMN `%s` TYPE VARCHAR(64)", alts[0], alts[1]))
			}
			if err != nil {
				return fmt.Errorf("alter column '%s' of table '%s' failed: %w", alts[1], alts[0], err)
			}
		}

		if setting.Database.Type.IsMSSQL() {
			recreateIndexes := []string{
				"CREATE INDEX IDX_commit_status_context_hash ON commit_status(context_hash)",
				"CREATE UNIQUE INDEX UQE_review_state_pull_commit_user ON review_state(user_id, pull_id, commit_sha)",
				"CREATE UNIQUE INDEX UQE_repo_archiver_s ON repo_archiver(repo_id, type, commit_id)",
			}
			for _, s := range recreateIndexes {
				_, err := db.Exec(s)
				if err != nil {
					return errors.New(s + " " + err.Error())
				}
			}
		}
	}
	log.Debug("Updated database tables to hold SHA256 git hash references")

	return db.Commit()
}

func addObjectFormatNameToRepository(x *xorm.Engine) error {
	type Repository struct {
		ObjectFormatName string `xorm:"VARCHAR(6) NOT NULL DEFAULT 'sha1'"`
	}

	if err := x.Sync(new(Repository)); err != nil {
		return err
	}

	// Here to catch weird edge-cases where column constraints above are
	// not applied by the DB backend
	_, err := x.Exec("UPDATE repository set object_format_name = 'sha1' WHERE object_format_name = '' or object_format_name IS NULL")
	return err
}

func AdjustDBForSha256(x *xorm.Engine) error {
	if err := expandHashReferencesToSha256(x); err != nil {
		return err
	}
	return addObjectFormatNameToRepository(x)
}
