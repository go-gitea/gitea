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

// getSchedulesCtx returns the repository to scope to, zero meaning site-wide
func getSchedulesCtx(ctx *context.Context) (repoID int64, tpl templates.TplName, err error) {
	if ctx.Data["PageIsRepoSettings"] == true {
		return ctx.Repo.Repository.ID, tplRepoSchedules, nil
	}
	if ctx.Data["PageIsAdmin"] == true {
		return 0, tplAdminSchedules, nil
	}
	return 0, "", errors.New("unable to set schedules context")
}

type scheduleRow struct {
	Schedule *actions_model.ActionSchedule
	Repo     *repo_model.Repository
	Spec     string
	SpecRow  *actions_model.ActionScheduleSpec // nil when the cron expression could not be parsed
}

func Schedules(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.schedules")
	ctx.Data["PageType"] = "schedules"
	ctx.Data["PageIsSharedSettingsSchedules"] = true

	repoID, tpl, err := getSchedulesCtx(ctx)
	if err != nil {
		ctx.ServerError("getSchedulesCtx", err)
		return
	}
	isRepo := repoID > 0

	page := max(ctx.FormInt("page"), 1)
	pageSize := 50

	schedules, count, err := db.FindAndCount[actions_model.ActionSchedule](ctx, actions_model.FindScheduleOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: pageSize,
		},
		RepoID: repoID,
	})
	if err != nil {
		ctx.ServerError("FindAndCount[ActionSchedule]", err)
		return
	}

	repos := make(map[int64]*repo_model.Repository)
	if !isRepo {
		repoIDs := make([]int64, 0, len(schedules))
		for _, s := range schedules {
			repoIDs = append(repoIDs, s.RepoID)
		}
		if repos, err = repo_model.GetRepositoriesMapByIDs(ctx, repoIDs); err != nil {
			ctx.ServerError("GetRepositoriesMapByIDs", err)
			return
		}
	}

	scheduleIDs := make([]int64, 0, len(schedules))
	for _, s := range schedules {
		scheduleIDs = append(scheduleIDs, s.ID)
	}
	var specs []*actions_model.ActionScheduleSpec
	if len(scheduleIDs) > 0 {
		if specs, err = db.Find[actions_model.ActionScheduleSpec](ctx, actions_model.FindSpecOptions{ScheduleIDs: scheduleIDs}); err != nil {
			ctx.ServerError("Find[ActionScheduleSpec]", err)
			return
		}
	}

	specsMap := make(map[int64]map[string][]*actions_model.ActionScheduleSpec, len(schedules))
	for _, spec := range specs {
		if specsMap[spec.ScheduleID] == nil {
			specsMap[spec.ScheduleID] = make(map[string][]*actions_model.ActionScheduleSpec)
		}
		specsMap[spec.ScheduleID][spec.Spec] = append(specsMap[spec.ScheduleID][spec.Spec], spec)
	}

	// one row per configured cron expression, so every expression shows its own next and last run
	rows := make([]scheduleRow, 0, len(schedules))
	for _, s := range schedules {
		for _, spec := range s.Specs {
			row := scheduleRow{Schedule: s, Repo: repos[s.RepoID], Spec: spec}
			// a spec row exists only for expressions that parsed, see CreateScheduleTask
			if matches := specsMap[s.ID][spec]; len(matches) > 0 {
				row.SpecRow = matches[0]
				specsMap[s.ID][spec] = matches[1:]
			}
			rows = append(rows, row)
		}
	}

	ctx.Data["Schedules"] = rows
	ctx.Data["Total"] = count
	ctx.Data["IsRepoSchedules"] = isRepo

	pager := context.NewPagination(count, pageSize, page, 5)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tpl)
}
