// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

type Star struct {
	StarListID int64 `xorm:"UNIQUE(s)"`
}

type StarList struct {
	ID   int64 `xorm:"pk autoincr"`
	UID  int64 `xorm:"INDEX"`
	Name string
	Desc string

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// TableName return database table name for xorm
func (StarList) TableName() string {
	return "star_list"
}

func AddStarList(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	err := sess.Sync(new(Star))
	if err != nil {
		return err
	}
	err = sess.Sync(new(StarList))
	if err != nil {
		return err
	}
	return sess.Commit()
}
