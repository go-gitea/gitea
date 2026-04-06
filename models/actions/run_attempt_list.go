// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"

	"xorm.io/builder"
)

type RunAttemptList []*RunAttempt

// GetUserIDs returns a slice of user's id
func (attempts RunAttemptList) GetUserIDs() []int64 {
	return container.FilterSlice(attempts, func(attempt *RunAttempt) (int64, bool) {
		return attempt.TriggerUserID, true
	})
}

func (attempts RunAttemptList) LoadTriggerUser(ctx context.Context) error {
	userIDs := attempts.GetUserIDs()
	users := make(map[int64]*user_model.User, len(userIDs))
	if err := db.GetEngine(ctx).In("id", userIDs).Find(&users); err != nil {
		return err
	}
	for _, attempt := range attempts {
		if attempt.TriggerUserID == user_model.ActionsUserID {
			attempt.TriggerUser = user_model.NewActionsUser()
		} else {
			attempt.TriggerUser = users[attempt.TriggerUserID]
			if attempt.TriggerUser == nil {
				attempt.TriggerUser = user_model.NewGhostUser()
			}
		}
	}
	return nil
}

type FindRunAttemptOptions struct {
	db.ListOptions
	RepoID           int64
	RunID            int64
	Attempt          int64
	Statuses         []Status
	ConcurrencyGroup string
}

func (opts FindRunAttemptOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"`action_run_attempt`.repo_id": opts.RepoID})
	}
	if opts.RunID > 0 {
		cond = cond.And(builder.Eq{"`action_run_attempt`.run_id": opts.RunID})
	}
	if opts.Attempt > 0 {
		cond = cond.And(builder.Eq{"`action_run_attempt`.attempt": opts.Attempt})
	}
	if len(opts.Statuses) > 0 {
		cond = cond.And(builder.In("`action_run_attempt`.status", opts.Statuses))
	}
	if opts.ConcurrencyGroup != "" {
		cond = cond.And(builder.Eq{"`action_run_attempt`.concurrency_group": opts.ConcurrencyGroup})
	}
	return cond
}

func (opts FindRunAttemptOptions) ToOrders() string {
	return "`action_run_attempt`.`attempt` DESC"
}
