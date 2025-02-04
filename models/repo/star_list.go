// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// StarList ...
type StarList struct {
	ID   int64 `xorm:"pk autoincr"`
	UID  int64 `xorm:"INDEX"`
	Name string
	Desc string

	StarIDs []int64      `xorm:"-"`
	Repos   []Repository `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(StarList))
}

func (list *StarList) GetCount() int64 {
	var star Star
	count, err := db.GetEngine(context.Background()).Where("star_list_id = ?", list.ID).Count(star)
	if err != nil {
		return 0
	}
	return count
}

func (list *StarList) LoadStars() {
	var star []Star
	err := db.GetEngine(context.Background()).Where("star_list_id = ?", list.ID).Find(&star)
	if err != nil {
		return
	}
	for _, star := range star {
		list.StarIDs = append(list.StarIDs, star.ID)
	}
}

func GetStarListByUID(ctx context.Context, uid int64) ([]*StarList, error) {
	var list []*StarList
	err := db.GetEngine(ctx).Where("uid = ?", uid).Find(&list)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func GetReposByStarListID(ctx context.Context, starListID int64) ([]*Repository, error) {
	repos := make([]*Repository, 0, 100)
	err := db.GetEngine(ctx).Where("star_list_id = ?", starListID).Find(&repos)
	if err != nil {
		return nil, err
	}
	return repos, nil
}
