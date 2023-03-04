// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func renameTaskErrorsToMessage(x *xorm.Engine) error {
	type Task struct {
		Errors string `xorm:"TEXT"` // if task failed, saved the error reason
		Type   int
		Status int `xorm:"index"`
	}

	// This migration maybe rerun so that we should check if it has been run
	messageExist, err := x.Dialect().IsColumnExist(x.DB(), context.Background(), "task", "message")
	if err != nil {
		return err
	}

	if messageExist {
		errorsExist, err := x.Dialect().IsColumnExist(x.DB(), context.Background(), "task", "errors")
		if err != nil {
			return err
		}
		if !errorsExist {
			return nil
		}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync2(new(Task)); err != nil {
		return fmt.Errorf("error on Sync2: %w", err)
	}

	if messageExist {
		// if both errors and message exist, drop message at first
		if err := dropTableColumns(sess, "task", "message"); err != nil {
			return err
		}
	}

	switch {
	case setting.Database.UseMySQL:
		if _, err := sess.Exec("ALTER TABLE `task` CHANGE errors message text"); err != nil {
			return err
		}
	case setting.Database.UseMSSQL:
		if _, err := sess.Exec("sp_rename 'task.errors', 'message', 'COLUMN'"); err != nil {
			return err
		}
	default:
		if _, err := sess.Exec("ALTER TABLE `task` RENAME COLUMN errors TO message"); err != nil {
			return err
		}
	}
	return sess.Commit()
}
