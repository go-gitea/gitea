// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package builds

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/core"
	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/util"
)

const (
	tplListBuilds base.TplName = "repo/builds/list"
	tplViewBuild  base.TplName = "repo/builds/view"
)

// MustEnableBuilds check if builds are enabled in settings
func MustEnableBuilds(ctx *context.Context) {
	if unit.TypeBuilds.UnitGlobalDisabled() {
		ctx.NotFound("EnableTypeBuilds", nil)
		return
	}

	if ctx.Repo.Repository != nil {
		if !ctx.Repo.CanRead(unit.TypeBuilds) {
			ctx.NotFound("MustEnableBuilds", nil)
			return
		}
	}
}

func List(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.builds")
	ctx.Data["PageIsBuildList"] = true

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	opts := bots_model.FindBuildOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
		},
		RepoID: ctx.Repo.Repository.ID,
	}
	if ctx.FormString("state") == "closed" {
		opts.IsClosed = util.OptionalBoolTrue
	} else {
		opts.IsClosed = util.OptionalBoolFalse
	}
	builds, err := bots_model.FindBuilds(opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	if err := builds.LoadTriggerUser(); err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	total, err := bots_model.CountBuilds(opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.Data["Builds"] = builds

	pager := context.NewPagination(int(total), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplListBuilds)
}

func ViewBuild(ctx *context.Context) {
	index := ctx.ParamsInt64("index")
	build, err := bots_model.GetBuildByRepoAndIndex(ctx.Repo.Repository.ID, index)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.Data["Name"] = build.Name + " - " + ctx.Tr("repo.builds")
	ctx.Data["PageIsBuildList"] = true
	ctx.Data["Build"] = build
	statuses, err := bots_model.GetBuildWorkflows(build.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.Data["WorkflowsStatuses"] = statuses

	ctx.HTML(http.StatusOK, tplViewBuild)
}

func GetBuildJobLogs(ctx *context.Context) {
	index := ctx.ParamsInt64("index")
	build, err := bots_model.GetBuildByRepoAndIndex(ctx.Repo.Repository.ID, index)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	workflows, err := bots_model.GetBuildWorkflows(build.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	var buildJob *bots_model.BuildStage
	wf := ctx.Params("workflow")
	jobname := ctx.Params("jobname")
LOOP_WORKFLOWS:
	for workflow, jobs := range workflows {
		if workflow == wf {
			for _, job := range jobs {
				if jobname == job.Name {
					buildJob = job
					break LOOP_WORKFLOWS
				}
			}
		}
	}
	if buildJob == nil {
		ctx.Error(http.StatusNotFound, fmt.Sprintf("workflow %s job %s not exist", wf, jobname))
		return
	}

	// TODO: if buildJob.LogToFile is true, read the logs from the file

	logs, err := bots_model.GetBuildLogs(build.ID, buildJob.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, logs)
}

type RunState struct {
	Title           string              `json:"title"`
	Jobs            []*RunStateJob      `json:"jobs"`
	CurrentJobInfo  *RunStateJobInfo    `json:"current_job_info"`
	CurrentJobSteps []*RunStateJobSteps `json:"current_job_steps"`
}

type RunStateJob struct {
	ID     int64            `json:"id"`
	Name   string           `json:"name"`
	Status core.BuildStatus `json:"status"`
}

type RunStateJobInfo struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

type RunStateJobSteps struct {
	Summary  string           `json:"summary"`
	Status   core.BuildStatus `json:"status"`
	Duration int64            `json:"duration"` // seconds
}

func GetRunState(ctx *context.Context) {
	runID := ctx.ParamsInt64("index")
	currentJobID := ctx.ParamsInt64("jobid")

	run, err := bots_model.GetRunByID(ctx, runID)
	if err != nil {
		if _, ok := err.(bots_model.ErrRunNotExist); ok {
			ctx.Error(http.StatusNotFound, err.Error())
			return
		}
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	jobs, err := bots_model.GetRunJobsByRunID(ctx, run.ID)
	if err != nil {
		if _, ok := err.(bots_model.ErrRunJobNotExist); ok {
			ctx.Error(http.StatusNotFound, err.Error())
			return
		}
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	state := &RunState{
		Title: run.Name,
		Jobs:  make([]*RunStateJob, len(jobs)),
	}
	for i, v := range jobs {
		state.Jobs[i] = &RunStateJob{
			ID:     v.ID,
			Name:   v.Name,
			Status: v.Status,
		}
	}
	if currentJobID != 0 {
		for _, job := range jobs {
			if job.ID == currentJobID {
				state.CurrentJobInfo = &RunStateJobInfo{
					Title: job.Name,
				}
				if job.TaskID == 0 {
					state.CurrentJobInfo.Detail = "wait to be pick up by a runner"
				}
				state.CurrentJobInfo.Detail = "TODO: more detail info" // TODO: more detail info, need to be internationalized

				task, err := bots_model.GetTaskByID(ctx, job.TaskID)
				if err != nil {
					ctx.Error(http.StatusInternalServerError, err.Error())
					return
				}
				task.Job = job
				if err := task.LoadAttributes(ctx); err != nil {
					ctx.Error(http.StatusInternalServerError, err.Error())
					return
				}
				state.CurrentJobSteps = make([]*RunStateJobSteps, 0, len(task.Steps)+2)
				// TODO: add steps "Set up job" and "Complete job"
				for _, step := range task.Steps {
					state.CurrentJobSteps = append(state.CurrentJobSteps, &RunStateJobSteps{
						Summary:  step.Name,
						Status:   core.StatusRunning, // TODO: add status to step
						Duration: int64(step.Stopped - step.Started),
					})
				}
			}
		}
	}

	ctx.JSON(http.StatusOK, state)
}

func GetRunLog(ctx *context.Context) {
	// TODO
}
