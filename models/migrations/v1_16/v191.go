// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16 //nolint

import (
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func AlterIssueAndCommentTextFieldsToLongText(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if setting.Database.UseMySQL {
		if _, err := sess.Exec("ALTER TABLE `issue` CHANGE `content` `content` LONGTEXT"); err != nil {
			return err
		}
		if _, err := sess.Exec("ALTER TABLE `comment` CHANGE `content` `content` LONGTEXT, CHANGE `patch` `patch` LONGTEXT"); err != nil {
			return err
		}
	}
	return sess.Commit()
}
