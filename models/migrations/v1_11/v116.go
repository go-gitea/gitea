// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_11 //nolint

import (
	"xorm.io/xorm"
)

func ExtendTrackedTimes(x *xorm.Engine) error {
	type TrackedTime struct {
		Time    int64 `xorm:"NOT NULL"`
		Deleted bool  `xorm:"NOT NULL DEFAULT false"`
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Exec("DELETE FROM tracked_time WHERE time IS NULL"); err != nil {
		return err
	}

	if err := sess.Sync(new(TrackedTime)); err != nil {
		return err
	}

	return sess.Commit()
}
