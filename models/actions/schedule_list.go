// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"

	"xorm.io/builder"
)

type ScheduleList []*ActionSchedule

// GetUserIDs returns a slice of user's id
func (schedules ScheduleList) GetUserIDs() []int64 {
	return container.FilterSlice(schedules, func(schedule *ActionSchedule) (int64, bool) {
		return schedule.TriggerUserID, true
	})
}

func (schedules ScheduleList) GetRepoIDs() []int64 {
	return container.FilterSlice(schedules, func(schedule *ActionSchedule) (int64, bool) {
		return schedule.RepoID, true
	})
}

func (schedules ScheduleList) LoadTriggerUser(ctx context.Context) error {
	userIDs := schedules.GetUserIDs()
	users := make(map[int64]*user_model.User, len(userIDs))
	if err := db.GetEngine(ctx).In("id", userIDs).Find(&users); err != nil {
		return err
	}
	for _, schedule := range schedules {
		if schedule.TriggerUserID == user_model.ActionsUserID {
			schedule.TriggerUser = user_model.NewActionsUser()
		} else {
			schedule.TriggerUser = users[schedule.TriggerUserID]
			if schedule.TriggerUser == nil {
				schedule.TriggerUser = user_model.NewGhostUser()
			}
		}
	}
	return nil
}

func (schedules ScheduleList) LoadRepos(ctx context.Context) error {
	repoIDs := schedules.GetRepoIDs()
	repos, err := repo_model.GetRepositoriesMapByIDs(ctx, repoIDs)
	if err != nil {
		return err
	}
	for _, schedule := range schedules {
		schedule.Repo = repos[schedule.RepoID]
	}
	return nil
}

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
