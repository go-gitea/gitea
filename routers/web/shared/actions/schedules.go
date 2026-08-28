// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"net/http"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/templates"
	shared_user "gitea.dev/routers/web/shared/user"
	"gitea.dev/services/context"
)

const (
	tplRepoSchedules  templates.TplName = "repo/settings/actions"
	tplOrgSchedules   templates.TplName = "org/settings/actions"
	tplUserSchedules  templates.TplName = "user/settings/actions"
	tplAdminSchedules templates.TplName = "admin/actions"
)

type schedulesCtx struct {
	RepoID  int64
	OwnerID int64
	Tpl     templates.TplName
}

func getSchedulesCtx(ctx *context.Context) (*schedulesCtx, error) {
	if ctx.Data["PageIsRepoSettings"] == true {
		return &schedulesCtx{RepoID: ctx.Repo.Repository.ID, Tpl: tplRepoSchedules}, nil
	}
	if ctx.Data["PageIsOrgSettings"] == true {
		if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
			return nil, err
		}
		return &schedulesCtx{OwnerID: ctx.ContextUser.ID, Tpl: tplOrgSchedules}, nil
	}
	if ctx.Data["PageIsUserSettings"] == true {
		return &schedulesCtx{OwnerID: ctx.Doer.ID, Tpl: tplUserSchedules}, nil
	}
	if ctx.Data["PageIsAdmin"] == true {
		return &schedulesCtx{Tpl: tplAdminSchedules}, nil
	}
	return nil, errors.New("unable to set schedules context")
}

type scheduleSpecRow struct {
	Spec string
	Row  *actions_model.ActionScheduleSpec // nil when the cron expression could not be parsed
}

type scheduleRow struct {
	Schedule *actions_model.ActionSchedule
	Repo     *repo_model.Repository
	Specs    []scheduleSpecRow
	// SkipReason is the locale key explaining why the scheduler ignores this schedule, empty when it runs
	SkipReason string
}

type specKey struct {
	scheduleID int64
	spec       string
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
	isRepo := sCtx.RepoID > 0

	page := max(ctx.FormInt("page"), 1)
	pageSize := 50

	schedules, count, err := db.FindAndCount[actions_model.ActionSchedule](ctx, actions_model.FindScheduleOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: pageSize,
		},
		RepoID:  sCtx.RepoID,
		OwnerID: sCtx.OwnerID,
	})
	if err != nil {
		ctx.ServerError("FindAndCount[ActionSchedule]", err)
		return
	}

	repoIDs := make([]int64, 0, len(schedules))
	scheduleIDs := make([]int64, 0, len(schedules))
	for _, s := range schedules {
		repoIDs = append(repoIDs, s.RepoID)
		scheduleIDs = append(scheduleIDs, s.ID)
	}

	var repos map[int64]*repo_model.Repository
	if isRepo {
		repos = map[int64]*repo_model.Repository{ctx.Repo.Repository.ID: ctx.Repo.Repository}
	} else if repos, err = repo_model.GetRepositoriesMapByIDs(ctx, repoIDs); err != nil {
		ctx.ServerError("GetRepositoriesMapByIDs", err)
		return
	}

	actionsUnits, err := repo_model.GetUnitsMapByRepoIDs(ctx, repoIDs, unit.TypeActions)
	if err != nil {
		ctx.ServerError("GetUnitsMapByRepoIDs", err)
		return
	}

	var specs []*actions_model.ActionScheduleSpec
	if len(scheduleIDs) > 0 {
		if specs, err = db.Find[actions_model.ActionScheduleSpec](ctx, actions_model.FindSpecOptions{
			ListOptions: db.ListOptionsAll,
			RepoID:      sCtx.RepoID,
			ScheduleIDs: scheduleIDs,
		}); err != nil {
			ctx.ServerError("Find[ActionScheduleSpec]", err)
			return
		}
	}

	specsByKey := make(map[specKey][]*actions_model.ActionScheduleSpec, len(specs))
	for _, spec := range specs {
		key := specKey{spec.ScheduleID, spec.Spec}
		specsByKey[key] = append(specsByKey[key], spec)
	}

	rows := make([]scheduleRow, 0, len(schedules))
	for _, s := range schedules {
		row := scheduleRow{
			Schedule:   s,
			Repo:       repos[s.RepoID],
			Specs:      make([]scheduleSpecRow, 0, len(s.Specs)),
			SkipReason: scheduleSkipReason(repos[s.RepoID], actionsUnits[s.RepoID], s.WorkflowID),
		}
		for _, spec := range s.Specs {
			entry := scheduleSpecRow{Spec: spec}
			key := specKey{s.ID, spec}
			// a spec row exists only for expressions that parsed, see CreateScheduleTask
			if matches := specsByKey[key]; len(matches) > 0 {
				entry.Row = matches[0]
				specsByKey[key] = matches[1:]
			}
			row.Specs = append(row.Specs, entry)
		}
		rows = append(rows, row)
	}

	ctx.Data["Schedules"] = rows
	ctx.Data["Total"] = count
	ctx.Data["IsRepoSchedules"] = isRepo

	pager := context.NewPagination(count, pageSize, page, 5)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, sCtx.Tpl)
}

// scheduleSkipReason mirrors the conditions startTasks skips a spec on, see services/actions/schedule_tasks.go
func scheduleSkipReason(repo *repo_model.Repository, actionsUnit *repo_model.RepoUnit, workflowID string) string {
	switch {
	case repo == nil:
		return ""
	case repo.IsArchived:
		return "actions.schedules.repo_archived"
	case actionsUnit == nil:
		return "actions.schedules.actions_disabled"
	case actionsUnit.ActionsConfig().IsWorkflowDisabled(workflowID):
		return "actions.schedules.workflow_disabled"
	}
	return ""
}
