// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/setting"
)

func AlterIssueAndCommentTextFieldsToLongText(x db.EngineMigration) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if setting.Database.Type.IsMySQL() {
		if _, err := sess.Exec("ALTER TABLE `issue` CHANGE `content` `content` LONGTEXT"); err != nil {
			return err
		}
		if _, err := sess.Exec("ALTER TABLE `comment` CHANGE `content` `content` LONGTEXT, CHANGE `patch` `patch` LONGTEXT"); err != nil {
			return err
		}
	}
	return sess.Commit()
}
