// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package v1_22

import (
	"context"
	"errors"
	"fmt"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	"xorm.io/xorm"
)

func expandHashReferencesToSha256(x base.EngineMigration) error {
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
				"DROP INDEX [IDX_commit_status_context_hash] ON [commit_status]",
				"DROP INDEX [UQE_review_state_pull_commit_user] ON [review_state]",
				"DROP INDEX [UQE_repo_archiver_s] ON [repo_archiver]",
			}
			for _, s := range droppedIndexes {
				_, err := db.Exec(s)
				if err != nil {
					return errors.New(s + " " + err.Error())
				}
			}
		}
		if setting.Database.Type.IsFirebird() {
			// drop indexes that need to be re-created afterwards, Firebird drops an index without a table
			droppedIndexes := []string{
				"DROP INDEX `IDX_commit_status_context_hash`",
				"DROP INDEX `UQE_review_state_pull_commit_user`",
				"DROP INDEX `UQE_repo_archiver_s`",
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
			} else if setting.Database.Type.IsFirebird() {
				// Firebird refuses to shrink a VARCHAR below its declared size even when
				// the data fits, so an alter that shrinks rebuilds the column through a
				// temporary one
				_, err = db.Exec(fmt.Sprintf("ALTER TABLE `%s` ALTER COLUMN `%s` TYPE VARCHAR(64)", alts[0], alts[1]))
				if err != nil {
					err = expandHashColumnOnFirebird(db, alts[0], alts[1])
				}
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
		if setting.Database.Type.IsFirebird() {
			recreateIndexes := []string{
				"CREATE INDEX `IDX_commit_status_context_hash` ON `commit_status` (`context_hash`)",
				"CREATE UNIQUE INDEX `UQE_review_state_pull_commit_user` ON `review_state` (`user_id`, `pull_id`, `commit_sha`)",
				"CREATE UNIQUE INDEX `UQE_repo_archiver_s` ON `repo_archiver` (`repo_id`, `type`, `commit_id`)",
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

// expandHashColumnOnFirebird rebuilds a hash column that Firebird refuses to shrink
// below its declared size: the data is copied through a temporary column, the original
// column is dropped and the copy renamed back. The index drops and re-creations around
// the alter loop keep the columns free of index references. DDL and DML cannot share a
// transaction on Firebird, so every phase commits before the next begins.
func expandHashColumnOnFirebird(db *xorm.Session, tableName, colName string) error {
	phases := [][]string{
		{fmt.Sprintf("ALTER TABLE `%s` ADD `tmp_expand_hash_col` VARCHAR(64)", tableName)},
		{fmt.Sprintf("UPDATE `%s` SET `tmp_expand_hash_col` = `%s`", tableName, colName)},
		{
			fmt.Sprintf("ALTER TABLE `%s` DROP `%s`", tableName, colName),
			fmt.Sprintf("ALTER TABLE `%s` ALTER COLUMN `tmp_expand_hash_col` TO `%s`", tableName, colName),
		},
	}
	for _, statements := range phases {
		for _, s := range statements {
			if _, err := db.Exec(s); err != nil {
				return errors.New(s + " " + err.Error())
			}
		}
		if err := db.Commit(); err != nil {
			return err
		}
		if err := db.Begin(); err != nil {
			return err
		}
	}
	return nil
}

func addObjectFormatNameToRepository(x base.EngineMigration) error {
	type Repository struct {
		ObjectFormatName string `xorm:"VARCHAR(6) NOT NULL DEFAULT 'sha1'"`
	}

	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreIndices:    true,
		IgnoreConstrains: true,
	}, new(Repository)); err != nil {
		return err
	}

	// Here to catch weird edge-cases where column constraints above are
	// not applied by the DB backend
	_, err := x.Exec("UPDATE `repository` set `object_format_name` = 'sha1' WHERE `object_format_name` = '' or `object_format_name` IS NULL")
	return err
}

func AdjustDBForSha256(_ context.Context, x base.EngineMigration) error {
	if err := expandHashReferencesToSha256(x); err != nil {
		return err
	}
	return addObjectFormatNameToRepository(x)
}
