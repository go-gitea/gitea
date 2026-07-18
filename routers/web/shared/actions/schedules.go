// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"net/http"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
)

const (
	tplRepoSchedules  templates.TplName = "repo/settings/actions"
	tplAdminSchedules templates.TplName = "admin/actions"
)

type schedulesCtx struct {
	RepoID   int64
	IsRepo   bool
	IsAdmin  bool
	Template templates.TplName
}

func getSchedulesCtx(ctx *context.Context) (*schedulesCtx, error) {
	if ctx.Data["PageIsRepoSettings"] == true {
		return &schedulesCtx{
			RepoID:   ctx.Repo.Repository.ID,
			IsRepo:   true,
			Template: tplRepoSchedules,
		}, nil
	}
	if ctx.Data["PageIsAdmin"] == true {
		return &schedulesCtx{
			IsAdmin:  true,
			Template: tplAdminSchedules,
		}, nil
	}
	return nil, errors.New("unable to set schedules context")
}

// scheduleInfo is a single schedule entry for display on the settings page.
type scheduleInfo struct {
	Schedule *actions_model.ActionSchedule
	Repo     *repo_model.Repository
	Specs    []*actions_model.ActionScheduleSpec
}

func Schedules(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.schedules")
	ctx.Data["PageType"] = "schedules"
	ctx.Data["PageIsSharedSettingsSchedules"] = true

	sCtx, err := getSchedulesCtx(ctx)
	if err != nil {
		ctx.ServerError("getSchedulesCtx", err)
		return
	}

	page := max(ctx.FormInt("page"), 1)
	pageSize := 50

	opts := actions_model.FindScheduleOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: pageSize,
		},
	}
	if sCtx.IsRepo {
		opts.RepoID = sCtx.RepoID
	}

	schedules, count, err := db.FindAndCount[actions_model.ActionSchedule](ctx, opts)
	if err != nil {
		ctx.ServerError("FindAndCount[ActionSchedule]", err)
		return
	}

	// Load repos for all schedules (for admin view; repo view always has one repo)
	repoIDs := make([]int64, 0, len(schedules))
	for _, s := range schedules {
		repoIDs = append(repoIDs, s.RepoID)
	}
	repos, err := repo_model.GetRepositoriesMapByIDs(ctx, repoIDs)
	if err != nil {
		ctx.ServerError("GetRepositoriesMapByIDs", err)
		return
	}

	// Load specs for all schedules on this page
	scheduleIDs := make([]int64, 0, len(schedules))
	for _, s := range schedules {
		scheduleIDs = append(scheduleIDs, s.ID)
	}
	var specs []*actions_model.ActionScheduleSpec
	if len(scheduleIDs) > 0 {
		if err := db.GetEngine(ctx).In("schedule_id", scheduleIDs).Find(&specs); err != nil {
			ctx.ServerError("Find[ActionScheduleSpec]", err)
			return
		}
	}

	// Group specs by ScheduleID
	specsMap := make(map[int64][]*actions_model.ActionScheduleSpec, len(specs))
	for _, spec := range specs {
		specsMap[spec.ScheduleID] = append(specsMap[spec.ScheduleID], spec)
	}

	infos := make([]scheduleInfo, 0, len(schedules))
	for _, s := range schedules {
		infos = append(infos, scheduleInfo{
			Schedule: s,
			Repo:     repos[s.RepoID],
			Specs:    specsMap[s.ID],
		})
	}

	ctx.Data["Schedules"] = infos
	ctx.Data["Total"] = count
	ctx.Data["IsRepoSchedules"] = sCtx.IsRepo

	pager := context.NewPagination(count, pageSize, page, 5)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, sCtx.Template)
}
