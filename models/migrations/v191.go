// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/setting"
	"xorm.io/xorm"
)

func alterIssueAndCommentTextFieldsToLongText(x *xorm.Engine) error {

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
