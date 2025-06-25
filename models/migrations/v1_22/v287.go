// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"xorm.io/xorm"
)

type BadgeUnique struct {
	ID   int64  `xorm:"pk autoincr"`
	Slug string `xorm:"UNIQUE"`
}

func (BadgeUnique) TableName() string {
	return "badge"
}

func UseSlugInsteadOfIDForBadges(x *xorm.Engine) error {
	type Badge struct {
		Slug string
	}

	err := x.Sync(new(Badge))
	if err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	_, err = sess.Exec("UPDATE `badge` SET `slug` = `id` Where `slug` IS NULL")
	if err != nil {
		return err
	}

	err = sess.Sync(new(BadgeUnique))
	if err != nil {
		return err
	}

	return sess.Commit()
}
