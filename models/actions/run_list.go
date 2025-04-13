// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/translation"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"xorm.io/builder"
)

type RunList []*ActionRun

// GetUserIDs returns a slice of user's id
func (runs RunList) GetUserIDs() []int64 {
	return container.FilterSlice(runs, func(run *ActionRun) (int64, bool) {
		return run.TriggerUserID, true
	})
}

func (runs RunList) GetRepoIDs() []int64 {
	return container.FilterSlice(runs, func(run *ActionRun) (int64, bool) {
		return run.RepoID, true
	})
}

func (runs RunList) LoadTriggerUser(ctx context.Context) error {
	userIDs := runs.GetUserIDs()
	users := make(map[int64]*user_model.User, len(userIDs))
	if err := db.GetEngine(ctx).In("id", userIDs).Find(&users); err != nil {
		return err
	}
	for _, run := range runs {
		if run.TriggerUserID == user_model.ActionsUserID {
			run.TriggerUser = user_model.NewActionsUser()
		} else {
			run.TriggerUser = users[run.TriggerUserID]
			if run.TriggerUser == nil {
				run.TriggerUser = user_model.NewGhostUser()
			}
		}
	}
	return nil
}

func (runs RunList) LoadRepos(ctx context.Context) error {
	repoIDs := runs.GetRepoIDs()
	repos, err := repo_model.GetRepositoriesMapByIDs(ctx, repoIDs)
	if err != nil {
		return err
	}
	for _, run := range runs {
		run.Repo = repos[run.RepoID]
	}
	return nil
}

type FindRunOptions struct {
	db.ListOptions
	RepoID        int64
	OwnerID       int64
	WorkflowID    string
	Ref           string // the commit/tag/… that caused this workflow
	TriggerUserID int64
	TriggerEvent  webhook_module.HookEventType
	Approved      bool // not util.OptionalBool, it works only when it's true
	Status        []Status
}

func (opts FindRunOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.OwnerID > 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	if opts.WorkflowID != "" {
		cond = cond.And(builder.Eq{"workflow_id": opts.WorkflowID})
	}
	if opts.TriggerUserID > 0 {
		cond = cond.And(builder.Eq{"trigger_user_id": opts.TriggerUserID})
	}
	if opts.Approved {
		cond = cond.And(builder.Gt{"approved_by": 0})
	}
	if len(opts.Status) > 0 {
		cond = cond.And(builder.In("status", opts.Status))
	}
	if opts.Ref != "" {
		cond = cond.And(builder.Eq{"ref": opts.Ref})
	}
	if opts.TriggerEvent != "" {
		cond = cond.And(builder.Eq{"trigger_event": opts.TriggerEvent})
	}
	return cond
}

func (opts FindRunOptions) ToOrders() string {
	return "`id` DESC"
}

type StatusInfo struct {
	Status          int
	DisplayedStatus string
}

// GetStatusInfoList returns a slice of StatusInfo
func GetStatusInfoList(ctx context.Context, lang translation.Locale) []StatusInfo {
	// same as those in aggregateJobStatus
	allStatus := []Status{StatusSuccess, StatusFailure, StatusWaiting, StatusRunning}
	statusInfoList := make([]StatusInfo, 0, 4)
	for _, s := range allStatus {
		statusInfoList = append(statusInfoList, StatusInfo{
			Status:          int(s),
			DisplayedStatus: s.LocaleString(lang),
		})
	}
	return statusInfoList
}

// GetActors returns a slice of Actors
func GetActors(ctx context.Context, repoID int64) ([]*user_model.User, error) {
	actors := make([]*user_model.User, 0, 10)

	return actors, db.GetEngine(ctx).Where(builder.In("id", builder.Select("`action_run`.trigger_user_id").From("`action_run`").
		GroupBy("`action_run`.trigger_user_id").
		Where(builder.Eq{"`action_run`.repo_id": repoID}))).
		Cols("id", "name", "full_name", "avatar", "avatar_email", "use_custom_avatar").
		OrderBy(user_model.GetOrderByName()).
		Find(&actors)
}
