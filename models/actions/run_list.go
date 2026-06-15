// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/translation"
	webhook_module "gitea.dev/modules/webhook"

	"xorm.io/builder"
)

type RunList []*ActionRun

func (runs RunList) LoadTriggerUser(ctx context.Context) error {
	userIDs := container.FilterSlice(runs, func(run *ActionRun) (int64, bool) {
		return run.TriggerUserID, run.TriggerUser == nil
	})
	users := make(map[int64]*user_model.User, len(userIDs))
	if err := db.GetEngine(ctx).In("id", userIDs).Find(&users); err != nil {
		return err
	}
	for _, run := range runs {
		if run.TriggerUser != nil {
			continue
		}
		run.TriggerUser = users[run.TriggerUserID]
		if run.TriggerUserID < 0 {
			run.TriggerUserID, run.TriggerUser, _ = user_model.GetPossibleUserByID(ctx, run.TriggerUserID)
		} else if run.TriggerUser == nil {
			run.TriggerUserID, run.TriggerUser, _ = user_model.GetPossibleUserByID(ctx, user_model.GhostUserID)
		}
	}
	return nil
}

func (runs RunList) LoadRepos(ctx context.Context) error {
	repoIDs := container.FilterSlice(runs, func(run *ActionRun) (int64, bool) {
		return run.RepoID, run.Repo == nil
	})
	repos, err := repo_model.GetRepositoriesMapByIDs(ctx, repoIDs)
	if err != nil {
		return err
	}
	for _, run := range runs {
		if run.Repo == nil {
			run.Repo = repos[run.RepoID]
		}
	}
	return nil
}

type FindRunOptions struct {
	db.ListOptions
	RepoID           int64
	OwnerID          int64
	WorkflowID       string
	Ref              string // the commit/tag/… that caused this workflow
	TriggerUserID    int64
	TriggerEvent     webhook_module.HookEventType
	Status           []Status
	ConcurrencyGroup string
	CommitSHA        string
}

func (opts FindRunOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"`action_run`.repo_id": opts.RepoID})
	}
	if opts.WorkflowID != "" {
		cond = cond.And(builder.Eq{"`action_run`.workflow_id": opts.WorkflowID})
	}
	if opts.TriggerUserID > 0 {
		cond = cond.And(builder.Eq{"`action_run`.trigger_user_id": opts.TriggerUserID})
	}
	if len(opts.Status) > 0 {
		cond = cond.And(builder.In("`action_run`.status", opts.Status))
	}
	if opts.Ref != "" {
		cond = cond.And(builder.Eq{"`action_run`.ref": opts.Ref})
	}
	if opts.TriggerEvent != "" {
		cond = cond.And(builder.Eq{"`action_run`.trigger_event": opts.TriggerEvent})
	}
	if opts.CommitSHA != "" {
		cond = cond.And(builder.Eq{"`action_run`.commit_sha": opts.CommitSHA})
	}
	return cond
}

func (opts FindRunOptions) ToJoins() []db.JoinFunc {
	if opts.OwnerID > 0 {
		return []db.JoinFunc{func(sess db.Engine) error {
			sess.Join("INNER", "repository", "repository.id = repo_id AND repository.owner_id = ?", opts.OwnerID)
			return nil
		}}
	}
	return nil
}

func (opts FindRunOptions) ToOrders() string {
	return "`action_run`.`id` DESC"
}

type StatusInfo struct {
	Status          int
	StatusName      string
	DisplayedStatus string
}

// GetStatusInfoList returns a slice of StatusInfo
func GetStatusInfoList(ctx context.Context, lang translation.Locale) []StatusInfo {
	// same as those in aggregateJobStatus
	allStatus := []Status{StatusSuccess, StatusFailure, StatusWaiting, StatusRunning, StatusCancelling}
	statusInfoList := make([]StatusInfo, 0, len(allStatus))
	for _, s := range allStatus {
		statusInfoList = append(statusInfoList, StatusInfo{
			Status:          int(s),
			StatusName:      s.String(),
			DisplayedStatus: s.LocaleString(lang),
		})
	}
	return statusInfoList
}

// GetRunBranches returns branch names for the run-list "Branch" filter.
// Sourced from the `branch` table (indexed by repo_id) rather than DISTINCT-ing
// `action_run.ref`, which is wildcard-matched and slow on large repos; as a side
// effect the list reflects existing branches, not only ones that produced a run.
func GetRunBranches(ctx context.Context, repoID int64) ([]string, error) {
	branches := make([]string, 0, 10)
	return branches, db.GetEngine(ctx).Table("branch").
		Where("repo_id = ?", repoID).
		And("is_deleted = ?", false).
		Cols("name").
		OrderBy("name ASC").
		Find(&branches)
}

// GetRunWorkflowIDs returns all distinct WorkflowIDs that have at least
// one ActionRun in the given repo.
func GetRunWorkflowIDs(ctx context.Context, repoID int64) ([]string, error) {
	ids := make([]string, 0, 10)
	return ids, db.GetEngine(ctx).Table("action_run").
		Where(builder.Eq{"repo_id": repoID}).
		Distinct("workflow_id").
		Cols("workflow_id").
		Asc("workflow_id").
		Find(&ids)
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
