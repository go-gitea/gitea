// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"code.gitea.io/gitea/models/migrations/base"

	"xorm.io/xorm"
)

type BadgeUnique struct {
	Slug string `xorm:"UNIQUE"`
}

func (BadgeUnique) TableName() string {
	return "badge"
}

func UseSlugInsteadOfIDForBadges(x *xorm.Engine) error {
	type Badge struct {
		Slug string
	}
	type UserBadge struct {
		BadgeSlug string `xorm:"INDEX"`
	}
	err := x.Sync(new(Badge), new(UserBadge))
	if err != nil {
		return err
	}
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	// add slug to each badge
	_, err = sess.Exec("UPDATE `badge` SET `slug` = `id`")
	if err != nil {
		return err
	}
	// add slug to each user badge
	_, err = sess.Exec("UPDATE `user_badge` SET `badge_slug` = (SELECT `slug` FROM `badge` WHERE `badge`.`id` = `user_badge`.`badge_id`)")
	if err != nil {
		return err
	}
	// drop badge_id columns from tables
	if err := base.DropTableColumns(sess, "user_badge", "badge_id"); err != nil {
		return err
	}
	if err := base.DropTableColumns(sess, "badge", "id"); err != nil {
		return err
	}
	err = sess.Sync(new(BadgeUnique))
	if err != nil {
		return err
	}
	return sess.Commit()
}
