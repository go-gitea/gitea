// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/builder"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func recreateUserTableToFixDefaultValues(x *xorm.Engine) error {
	switch x.Dialect().URI().DBType {
	case schemas.MYSQL:
		_, err := x.Exec("ALTER TABLE `user` MODIFY COLUMN keep_activity_private tinyint(1) DEFAULT 0 NOT NULL;")
		return err
	case schemas.POSTGRES:
		if _, err := x.Exec("ALTER TABLE `user` ALTER COLUMN keep_activity_private SET NOT NULL;"); err != nil {
			return err
		}
		_, err := x.Exec("ALTER TABLE `user` ALTER COLUMN keep_activity_private SET DEFAULT false;")
		return err
	}

	type User struct {
		ID                  int64 `xorm:"pk autoincr"`
		KeepActivityPrivate bool  `xorm:"NOT NULL DEFAULT false"`
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	var activityPrivateUsers []int64
	if err := sess.Select("id").Table("user").Where(builder.Eq{"keep_activity_private": true}).Find(&activityPrivateUsers); err != nil {
		return err
	}

	if err := dropTableColumns(sess, "user", "keep_activity_private"); err != nil {
		return err
	}

	if err := sess.Sync2(new(User)); err != nil {
		return err
	}

	for _, uid := range activityPrivateUsers {
		if _, err := sess.ID(uid).Cols("keep_activity_private").Update(&User{KeepActivityPrivate: true}); err != nil {
			return err
		}
	}

	return sess.Commit()
}
