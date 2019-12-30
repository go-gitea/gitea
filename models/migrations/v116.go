// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models"

	"xorm.io/xorm"
)

func extendTrackedTimes(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Exec("DELETE FROM tracked_time WHERE time IS NULL"); err != nil {
		return err
	}

	if err := sess.Sync2(new(models.TrackedTime)); err != nil {
		return err
	}

	return sess.Commit()
}
