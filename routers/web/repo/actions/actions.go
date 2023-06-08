// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"bytes"
	"fmt"
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/convert"

	"github.com/nektos/act/pkg/model"
)

const (
	tplListActions base.TplName = "repo/actions/list"
	tplViewActions base.TplName = "repo/actions/view"
)

type Workflow struct {
	Entry  git.TreeEntry
	ErrMsg string
}

// MustEnableActions check if actions are enabled in settings
func MustEnableActions(ctx *context.Context) {
	if !setting.Actions.Enabled {
		ctx.NotFound("MustEnableActions", nil)
		return
	}

	if unit.TypeActions.UnitGlobalDisabled() {
		ctx.NotFound("MustEnableActions", nil)
		return
	}

	if ctx.Repo.Repository != nil {
		if !ctx.Repo.CanRead(unit.TypeActions) {
			ctx.NotFound("MustEnableActions", nil)
			return
		}
	}
}

type StatusInfo struct {
	Status          int
	DisplayedStatus string
}

// getStatusInfos returns a slice of StatusInfo
func getStatusInfos(ctx *context.Context) []StatusInfo {
	statusInfos := make([]StatusInfo, 0, 7)
	for s := actions_model.StatusSuccess; s <= actions_model.StatusBlocked; s++ {
		statusInfos = append(statusInfos, StatusInfo{
			Status:          int(s),
			DisplayedStatus: s.String(),
		})
	}
	return statusInfos
}

func List(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsActions"] = true

	var workflows []Workflow
	if empty, err := ctx.Repo.GitRepo.IsEmpty(); err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	} else if !empty {
		commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
		entries, err := actions.ListWorkflows(commit)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}

		// Get all runner labels
		opts := actions_model.FindRunnerOptions{
			RepoID:        ctx.Repo.Repository.ID,
			WithAvailable: true,
		}
		runners, err := actions_model.FindRunners(ctx, opts)
		if err != nil {
			ctx.ServerError("FindRunners", err)
			return
		}
		allRunnerLabels := make(container.Set[string])
		for _, r := range runners {
			allRunnerLabels.AddMultiple(r.AgentLabels...)
		}

		workflows = make([]Workflow, 0, len(entries))
		for _, entry := range entries {
			workflow := Workflow{Entry: *entry}
			content, err := actions.GetContentFromEntry(entry)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, err.Error())
				return
			}
			wf, err := model.ReadWorkflow(bytes.NewReader(content))
			if err != nil {
				workflow.ErrMsg = ctx.Locale.Tr("actions.runs.invalid_workflow_helper", err.Error())
				workflows = append(workflows, workflow)
				continue
			}
			// Check whether have matching runner
			for _, j := range wf.Jobs {
				runsOnList := j.RunsOn()
				for _, ro := range runsOnList {
					if !allRunnerLabels.Contains(ro) {
						workflow.ErrMsg = ctx.Locale.Tr("actions.runs.no_matching_runner_helper", ro)
						break
					}
				}
				if workflow.ErrMsg != "" {
					break
				}
			}
			workflows = append(workflows, workflow)
		}
	}
	ctx.Data["workflows"] = workflows
	ctx.Data["RepoLink"] = ctx.Repo.Repository.Link()

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	workflow := ctx.FormString("workflow")
	actorID := ctx.FormInt64("actor")
	status := ctx.FormInt("status")
	ctx.Data["CurWorkflow"] = workflow
	ctx.Data["CurActor"] = actorID
	ctx.Data["CurStatus"] = status
	if actorID > 0 || status > int(actions_model.StatusUnknown) {
		ctx.Data["IsFiltered"] = true
	}

	opts := actions_model.FindRunOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
		},
		RepoID:           ctx.Repo.Repository.ID,
		WorkflowFileName: workflow,
		TriggerUserID:    actorID,
		Status:           actions_model.Status(status),
	}

	runs, total, err := actions_model.FindRuns(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	for _, run := range runs {
		run.Repo = ctx.Repo.Repository
	}

	if err := runs.LoadTriggerUser(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.Data["Runs"] = runs

	// Get all runs under the repo to get all actors and statuses
	allRunsOpts := actions_model.FindRunOptions{
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		RepoID: ctx.Repo.Repository.ID,
	}

	allRuns, _, err := actions_model.FindRuns(ctx, allRunsOpts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	actors, err := allRuns.GetActors(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	ctx.Data["Actors"] = actors

	ctx.Data["StatusInfos"] = getStatusInfos(ctx)

	pager := context.NewPagination(int(total), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	pager.AddParamString("workflow", workflow)
	pager.AddParamString("actor", fmt.Sprint(actorID))
	pager.AddParamString("status", fmt.Sprint(status))
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplListActions)
}
