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
	type User struct {
		ID                  int64 `xorm:"pk autoincr"`
		KeepActivityPrivate bool  `xorm:"NOT NULL DEFAULT false"`
		TmpCol              bool  `xorm:"NOT NULL DEFAULT false"`
	}

	if _, err := x.Where(builder.IsNull{"keep_activity_private"}).
		Cols("keep_activity_private").
		Update(User{KeepActivityPrivate: false}); err != nil {
		return err
	}

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
	case schemas.MSSQL:
		if _, err := x.Exec("ALTER TABLE `user` ADD  DEFAULT 0 FOR keep_activity_private GO;"); err != nil {
			return err
		}
		_, err := x.Exec("ALTER TABLE `user` ALTER COLUMN keep_activity_private bit NOT NULL GO;")
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync2(new(User)); err != nil {
		return err
	}

	if _, err := sess.Exec("UPDATE `user` SET tmp_col=keep_activity_private;"); err != nil {
		return err
	}

	if err := dropTableColumns(sess, "user", "keep_activity_private"); err != nil {
		return err
	}

	if err := sess.Sync2(new(User)); err != nil {
		return err
	}

	if _, err := sess.Exec("UPDATE `user` SET keep_activity_private=tmp_col;"); err != nil {
		return err
	}

	if err := dropTableColumns(sess, "user", "tmp_col"); err != nil {
		return err
	}

	return sess.Commit()
}
