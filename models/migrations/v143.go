// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"xorm.io/xorm"
)

func recalculateStars(x *xorm.Engine) (err error) {
	// because of issue https://github.com/go-gitea/gitea/issues/11949,
	// recalculate Stars number for all users to fully fix it.

	userIDs := make([]int64, 0)
	err = x.SQL("SELECT id FROM `user` WHERE type = 0").Find(&userIDs)
	if err != nil {
		return
	}

	var number int64

	for _, uid := range userIDs {
		if number, err = x.Where("uid = ?", uid).Count(new(models.Star)); err != nil {
			return
		}

		if _, err = x.Exec("UPDATE `user` SET num_stars=? WHERE id = ?", number, uid); err != nil {
			return err
		}
	}

	log.Debug("recalculate Stars number for all user finished")

	return err
}
