// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"code.gitea.io/gitea/models/db"

	"xorm.io/builder"
)

type ScheduleList []*ActionSchedule

type FindScheduleOptions struct {
	db.ListOptions
	RepoID  int64
	OwnerID int64
}

func (opts FindScheduleOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.OwnerID > 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}

	return cond
}

func (opts FindScheduleOptions) ToOrders() string {
	return "`id` DESC"
}
