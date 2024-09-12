// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13 //nolint

import (
	"code.gitea.io/gitea/modules/log"

	"xorm.io/xorm"
)

func RecalculateStars(x *xorm.Engine) (err error) {
	// because of issue https://github.com/go-gitea/gitea/issues/11949,
	// recalculate Stars number for all users to fully fix it.

	type User struct {
		ID int64 `xorm:"pk autoincr"`
	}

	const batchSize = 100
	sess := x.NewSession()
	defer sess.Close()

	for start := 0; ; start += batchSize {
		users := make([]User, 0, batchSize)
		if err := sess.Limit(batchSize, start).Where("type = ?", 0).Cols("id").Find(&users); err != nil {
			return err
		}
		if len(users) == 0 {
			break
		}

		if err := sess.Begin(); err != nil {
			return err
		}

		for _, user := range users {
			if _, err := sess.Exec("UPDATE `user` SET num_stars=(SELECT COUNT(*) FROM `star` WHERE uid=?) WHERE id=?", user.ID, user.ID); err != nil {
				return err
			}
		}

		if err := sess.Commit(); err != nil {
			return err
		}
	}

	log.Debug("recalculate Stars number for all user finished")

	return err
}
