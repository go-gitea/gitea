// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

const (
	tplProjects    base.TplName = "repo/projects/list"
	tplProjectsNew base.TplName = "repo/projects/new"

	projectTemplateKey = "ProjectTemplate"
)

func MustEnableProjects(ctx *context.Context) {

	if !setting.Repository.EnableKanbanBoard {
		ctx.NotFound("EnableKanbanBoard", nil)
		return
	}

	if !ctx.Repo.CanRead(models.UnitTypeProjects) {
		ctx.NotFound("MustEnableProjects", nil)
		return
	}
}

// Projects renders the home page of projects
func Projects(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.kanban_board")

	sortType := ctx.Query("sort")

	isShowClosed := ctx.Query("state") == "closed"
	repo := ctx.Repo.Repository
	page := ctx.QueryInt("page")
	if page <= 1 {
		page = 1
	}

	ctx.Data["OpenCount"] = repo.NumOpenProjects
	ctx.Data["ClosedCount"] = repo.NumClosedProjects

	var total int
	if !isShowClosed {
		total = int(repo.NumOpenProjects)
	} else {
		total = int(repo.NumClosedProjects)
	}

	projects, err := models.GetProjects(repo.ID, page, isShowClosed, sortType)
	if err != nil {
		ctx.ServerError("GetProjects", err)
		return
	}

	for i := range projects {
		projects[i].RenderedContent = string(markdown.Render([]byte(projects[i].Description), ctx.Repo.RepoLink, ctx.Repo.Repository.ComposeMetas()))
	}

	ctx.Data["Projects"] = projects

	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}

	pager := context.NewPagination(total, setting.UI.IssuePagingNum, page, 5)
	pager.AddParam(ctx, "state", "State")
	ctx.Data["Page"] = pager

	ctx.Data["IsShowClosed"] = isShowClosed
	ctx.Data["IsProjectsPage"] = true
	ctx.Data["SortType"] = sortType

	ctx.HTML(200, tplProjects)
}

// NewProject render creating a project page
func NewProject(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.projects.new")
	ctx.Data["ProjectTypes"] = models.GetProjectsConfig()

	ctx.HTML(200, tplProjectsNew)
}

// NewProjectPost creates a new project
func NewProjectPost(ctx *context.Context, form auth.CreateProjectForm) {

	ctx.Data["Title"] = ctx.Tr("repo.projects.new")

	if ctx.HasError() {
		ctx.HTML(200, tplProjectsNew)
		return
	}

	if err := models.NewProject(&models.Project{
		RepoID:      ctx.Repo.Repository.ID,
		Title:       form.Title,
		Description: form.Content,
		CreatorID:   ctx.User.ID,
		Type:        form.Type,
	}); err != nil {
		ctx.ServerError("NewProject", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.projects.create_success", form.Title))
	ctx.Redirect(ctx.Repo.RepoLink + "/projects")
}

// ChangeProjectStatus updates the status of a project between "open" and "close"
func ChangeProjectStatus(ctx *context.Context) {
	p, err := models.GetProjectByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrProjectNotExist(err) {
			ctx.NotFound("", err)
		} else {
			ctx.ServerError("GetProjectByRepoID", err)
		}
		return
	}

	switch ctx.Params(":action") {
	case "open":
		if p.IsClosed {
			if err = models.ChangeProjectStatus(p, false); err != nil {
				ctx.ServerError("ChangeProjectStatus", err)
				return
			}
		}
		ctx.Redirect(ctx.Repo.RepoLink + "/projects?state=open")
	case "close":
		if !p.IsClosed {
			p.ClosedDateUnix = util.TimeStampNow()
			if err = models.ChangeProjectStatus(p, true); err != nil {
				ctx.ServerError("ChangeProjectStatus", err)
				return
			}
		}
		ctx.Redirect(ctx.Repo.RepoLink + "/projects?state=closed")

	default:
		ctx.Redirect(ctx.Repo.RepoLink + "/projects")
	}
}

// DeleteProject delete a project
func DeleteProject(ctx *context.Context) {
	if err := models.DeleteProjectByRepoID(ctx.Repo.Repository.ID, ctx.QueryInt64("id")); err != nil {
		ctx.Flash.Error("DeleteProjectByRepoID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.projects.deletion_success"))
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/projects",
	})
}

// EditProject allows a project to be edited
func EditProject(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.projects.edit")
	ctx.Data["PageIsProjects"] = true
	ctx.Data["PageIsEditProjects"] = true

	p, err := models.GetProjectByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByRepoID", err)
		}
		return
	}

	ctx.Data["title"] = p.Title
	ctx.Data["content"] = p.Description

	ctx.HTML(200, tplProjectsNew)
}

// EditProjectPost response for edting a project
func EditProjectPost(ctx *context.Context, form auth.CreateProjectForm) {
	ctx.Data["Title"] = ctx.Tr("repo.projects.edit")
	ctx.Data["PageIsProjects"] = true
	ctx.Data["PageIsEditProjects"] = true

	if ctx.HasError() {
		ctx.HTML(200, tplMilestoneNew)
		return
	}

	p, err := models.GetProjectByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByRepoID", err)
		}
		return
	}

	p.Title = form.Title
	p.Description = form.Content
	if err = models.UpdateProject(p); err != nil {
		ctx.ServerError("UpdateProjects", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.projects.edit_success", p.Title))
	ctx.Redirect(ctx.Repo.RepoLink + "/projects")
}
