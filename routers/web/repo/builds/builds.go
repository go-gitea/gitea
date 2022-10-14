// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package builds

import (
	"fmt"
	"net/http"

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
