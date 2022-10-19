// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"

	"code.gitea.io/gitea/core"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
	"xorm.io/builder"
)

type RunList []*Run

// GetUserIDs returns a slice of user's id
func (runs RunList) GetUserIDs() []int64 {
	userIDsMap := make(map[int64]struct{})
	for _, run := range runs {
		userIDsMap[run.TriggerUserID] = struct{}{}
	}
	userIDs := make([]int64, 0, len(userIDsMap))
	for userID := range userIDsMap {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

func (runs RunList) LoadTriggerUser() error {
	userIDs := runs.GetUserIDs()
	users := make(map[int64]*user_model.User, len(userIDs))
	if err := db.GetEngine(db.DefaultContext).In("id", userIDs).Find(&users); err != nil {
		return err
	}
	for _, run := range runs {
		run.TriggerUser = users[run.TriggerUserID]
	}
	return nil
}

type FindRunOptions struct {
	db.ListOptions
	RepoID           int64
	IsClosed         util.OptionalBool
	WorkflowFileName string
}

func (opts FindRunOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.IsClosed.IsFalse() {
		cond = cond.And(builder.Eq{"status": core.StatusPending}.Or(
			builder.Eq{"status": core.StatusWaiting}.Or(
				builder.Eq{"status": core.StatusRunning})))
	} else if opts.IsClosed.IsTrue() {
		cond = cond.And(builder.Neq{"status": core.StatusPending}.And(
			builder.Neq{"status": core.StatusWaiting}.And(
				builder.Neq{"status": core.StatusRunning})))
	}
	if opts.WorkflowFileName != "" {
		cond = cond.And(builder.Eq{"workflow_id": opts.WorkflowFileName})
	}
	return cond
}

func FindRuns(ctx context.Context, opts FindRunOptions) (RunList, int64, error) {
	e := db.GetEngine(ctx).Where(opts.toConds())
	if opts.PageSize > 0 && opts.Page >= 1 {
		e.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	var runs RunList
	total, err := e.Desc("id").FindAndCount(&runs)
	return runs, total, err
}

func CountRuns(ctx context.Context, opts FindRunOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.toConds()).Count(new(Run))
}
