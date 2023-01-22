// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17 //nolint

import (
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func AlterHookTaskTextFieldsToLongText(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if setting.Database.UseMySQL {
		if _, err := sess.Exec("ALTER TABLE `hook_task` CHANGE `payload_content` `payload_content` LONGTEXT, CHANGE `request_content` `request_content` LONGTEXT, change `response_content` `response_content` LONGTEXT"); err != nil {
			return err
		}
	}
	return sess.Commit()
}
