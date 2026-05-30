// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/setting"
)

func AlterHookTaskTextFieldsToLongText(x db.EngineMigration) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if setting.Database.Type.IsMySQL() {
		if _, err := sess.Exec("ALTER TABLE `hook_task` CHANGE `payload_content` `payload_content` LONGTEXT, CHANGE `request_content` `request_content` LONGTEXT, change `response_content` `response_content` LONGTEXT"); err != nil {
			return err
		}
	}
	return sess.Commit()
}
