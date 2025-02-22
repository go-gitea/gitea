// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

type StarList struct {
	ID   int64 `xorm:"pk autoincr"`
	UID  int64 `xorm:"INDEX"`
	Name string
	Desc string

	Repos []Repository `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(StarList))
}

func InsertStarList(ctx context.Context, starList *StarList) error {
	_, err := db.GetEngine(ctx).Insert(starList)
	return err
}

func UpdateStarList(ctx context.Context, starList *StarList) error {
	_, err := db.GetEngine(ctx).Where("id = ?", starList.ID).AllCols().Update(starList)
	return err
}

func DeleteStarListByID(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).Delete(&StarList{ID: id})
	return err
}

func GetStarListByID(ctx context.Context, id int64) (*StarList, error) {
	starList := new(StarList)
	if has, err := db.GetEngine(ctx).Where("id = ?", id).Get(starList); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return starList, nil
}

func GetStarListsForUser(ctx context.Context, id int64) ([]*StarList, error) {
	starLists := make([]*StarList, 0, 10)
	err := db.GetEngine(ctx).Where("uid = ?", id).Find(&starLists)
	return starLists, err
}
