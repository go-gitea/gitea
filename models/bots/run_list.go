// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
	"xorm.io/builder"
)

type RunList []*Run

// GetUserIDs returns a slice of user's id
func (builds RunList) GetUserIDs() []int64 {
	userIDsMap := make(map[int64]struct{})
	for _, build := range builds {
		userIDsMap[build.TriggerUserID] = struct{}{}
	}
	userIDs := make([]int64, 0, len(userIDsMap))
	for userID := range userIDsMap {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

func (builds RunList) LoadTriggerUser() error {
	userIDs := builds.GetUserIDs()
	users := make(map[int64]*user_model.User, len(userIDs))
	if err := db.GetEngine(db.DefaultContext).In("id", userIDs).Find(&users); err != nil {
		return err
	}
	for _, task := range builds {
		task.TriggerUser = users[task.TriggerUserID]
	}
	return nil
}

type FindRunOptions struct {
	db.ListOptions
	RepoID   int64
	IsClosed util.OptionalBool
}

func (opts FindRunOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.IsClosed.IsFalse() {

	} else if opts.IsClosed.IsTrue() {

	}
	return cond
}

func FindRuns(ctx context.Context, opts FindRunOptions) (RunList, int64, error) {
	e := db.GetEngine(ctx).Where(opts.toConds())
	if opts.PageSize>0&&opts.Page >=1 {
		e.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	var runs RunList
	total, err := e.FindAndCount(&runs)
	return runs, total, err
}
