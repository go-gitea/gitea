// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

const (
	tplProjects           base.TplName = "repo/projects/list"
	tplProjectsNew        base.TplName = "repo/projects/new"
	tplProjectsView       base.TplName = "repo/projects/view"
	tplGenericProjectsNew base.TplName = "user/project"
)

// MustEnableProjects check if projects are enabled in settings
func MustEnableProjects(ctx *context.Context) {

	if models.UnitTypeProjects.UnitGlobalDisabled() {
		ctx.NotFound("EnableKanbanBoard", nil)
		return
	}

	if ctx.Repo.Repository != nil {
		if !ctx.Repo.CanRead(models.UnitTypeProjects) {
			ctx.NotFound("MustEnableProjects", nil)
			return
		}
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
		total = repo.NumOpenProjects
	} else {
		total = repo.NumClosedProjects
	}

	projects, err := models.GetProjects(models.ProjectSearchOptions{
		RepoID:   repo.ID,
		Page:     page,
		IsClosed: util.OptionalBoolOf(isShowClosed),
		SortType: sortType,
		Type:     models.RepositoryType,
	})
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

// NewRepoProjectPost creates a new project
func NewRepoProjectPost(ctx *context.Context, form auth.CreateProjectForm) {

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
		BoardType:   form.BoardType,
		Type:        models.RepositoryType,
	}); err != nil {
		ctx.ServerError("NewProject", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.projects.create_success", form.Title))
	ctx.Redirect(ctx.Repo.RepoLink + "/projects")
}

// ChangeProjectStatus updates the status of a project between "open" and "close"
// nolint: dupl
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
			p.ClosedDateUnix = timeutil.TimeStampNow()
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

// EditProjectPost response for editing a project
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

// ViewProject renders the kanban board for a project
func ViewProject(ctx *context.Context) {

	project, err := models.GetProjectByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByRepoID", err)
		}
		return
	}

	unCategorizedBoard, err := models.GetUnCategorizedBoard(ctx.Repo.Repository.ID, project.ID)
	unCategorizedBoard.Title = ctx.Tr("repo.projects.type.uncategorized")
	if err != nil {
		ctx.ServerError("GetUnCategorizedBoard", err)
		return
	}

	boards, err := models.GetProjectBoards(ctx.Repo.Repository.ID, project.ID)
	if err != nil {
		ctx.ServerError("GetProjectBoards", err)
		return
	}

	allBoards := models.ProjectBoards{unCategorizedBoard}
	allBoards = append(allBoards, boards...)

	if ctx.Data["Issues"], err = allBoards.LoadIssues(); err != nil {
		ctx.ServerError("LoadIssuesOfBoards", err)
		return
	}

	ctx.Data["Project"] = project
	ctx.Data["Boards"] = allBoards
	ctx.Data["PageIsProjects"] = true
	ctx.Data["RequiresDraggable"] = true

	ctx.HTML(200, tplProjectsView)
}

// UpdateIssueProject change an issue's project
func UpdateIssueProject(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	projectID := ctx.QueryInt64("id")
	for _, issue := range issues {
		oldProjectID := issue.ProjectID()
		if oldProjectID == projectID {
			continue
		}

		if err := models.ChangeProjectAssign(issue, ctx.User, projectID); err != nil {
			ctx.ServerError("ChangeProjectAssign", err)
			return
		}
	}

	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}

// DeleteProjectBoard allows for the deletion of a project board
func DeleteProjectBoard(ctx *context.Context) {
	if ctx.User == nil {
		ctx.JSON(403, map[string]string{
			"message": "Only signed in users are allowed to call make this action.",
		})
		return
	}

	if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(models.AccessModeWrite, models.UnitTypeProjects) {
		ctx.JSON(403, map[string]string{
			"message": "Only authorized users are allowed to call make this action.",
		})
		return
	}

	if err := models.DeleteProjectBoardByID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"), ctx.ParamsInt64(":boardID")); err != nil {
		ctx.ServerError("DeleteProjectBoardByID", err)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}

// AddBoardToProjectPost allows a new board to be added to a project.
func AddBoardToProjectPost(ctx *context.Context, form auth.EditProjectBoardTitleForm) {

	if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(models.AccessModeWrite, models.UnitTypeProjects) {
		ctx.JSON(403, map[string]string{
			"message": "Only authorized users are allowed to call make this action.",
		})
		return
	}

	projectID := ctx.ParamsInt64(":id")

	_, err := models.GetProjectByRepoID(ctx.Repo.Repository.ID, projectID)
	if err != nil {
		if models.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByRepoID", err)
		}
		return
	}

	if err := models.NewProjectBoard(&models.ProjectBoard{
		ProjectID: projectID,
		RepoID:    ctx.Repo.Repository.ID,
		Title:     form.Title,
		CreatorID: ctx.User.ID,
	}); err != nil {
		ctx.ServerError("NewProjectBoard", err)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}

// EditProjectBoardTitle allows a project board's title to be updated
func EditProjectBoardTitle(ctx *context.Context, form auth.EditProjectBoardTitleForm) {

	if ctx.User == nil {
		ctx.JSON(403, map[string]string{
			"message": "Only signed in users are allowed to call make this action.",
		})
		return
	}

	if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(models.AccessModeWrite, models.UnitTypeProjects) {
		ctx.JSON(403, map[string]string{
			"message": "Only authorized users are allowed to call make this action.",
		})
		return
	}

	board, err := models.GetProjectBoard(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"), ctx.ParamsInt64(":boardID"))
	if err != nil {
		if models.IsErrProjectBoardNotExist(err) {
			ctx.NotFound("", err)
		} else {
			ctx.ServerError("GetProjectBoard", err)
		}

		return
	}

	if form.Title != "" {
		board.Title = form.Title
	}

	if err := models.UpdateProjectBoard(board); err != nil {
		ctx.ServerError("UpdateProjectBoard", err)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}

// MoveIssueAcrossBoards move a card from one board to another in a project
func MoveIssueAcrossBoards(ctx *context.Context) {

	if ctx.User == nil {
		ctx.JSON(403, map[string]string{
			"message": "Only signed in users are allowed to call make this action.",
		})
		return
	}

	if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(models.AccessModeWrite, models.UnitTypeProjects) {
		ctx.JSON(403, map[string]string{
			"message": "Only authorized users are allowed to call make this action.",
		})
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

	var board *models.ProjectBoard

	if ctx.ParamsInt64(":boardID") == 0 {

		board = &models.ProjectBoard{
			ID:        0,
			ProjectID: 0,
			Title:     ctx.Tr("repo.projects.type.uncategorized"),
		}

	} else {
		board, err = models.GetProjectBoard(ctx.Repo.Repository.ID, p.ID, ctx.ParamsInt64(":boardID"))
		if err != nil {
			if models.IsErrProjectBoardNotExist(err) {
				ctx.NotFound("", nil)
			} else {
				ctx.ServerError("GetProjectBoard", err)
			}
			return
		}
	}

	issue, err := models.GetIssueByID(ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			fmt.Println(err)
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetIssueByID", err)
		}

		return
	}

	if err := models.MoveIssueAcrossProjectBoards(issue, board); err != nil {
		ctx.ServerError("MoveIssueAcrossProjectBoards", err)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}

// CreateProject renders the generic project creation page
func CreateProject(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.projects.new")
	ctx.Data["ProjectTypes"] = models.GetProjectsConfig()

	ctx.HTML(200, tplGenericProjectsNew)
}

// CreateProjectPost creates an individual and/or organization project
func CreateProjectPost(ctx *context.Context, form auth.UserCreateProjectForm) {

	user := checkContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}

	ctx.Data["ContextUser"] = user

	if ctx.HasError() {
		ctx.HTML(200, tplGenericProjectsNew)
		return
	}

	var projectType = models.IndividualType
	fmt.Println(user.IsOrganization())
	if user.IsOrganization() {
		projectType = models.OrganizationType
	}

	if err := models.NewProject(&models.Project{
		Title:       form.Title,
		Description: form.Content,
		CreatorID:   user.ID,
		BoardType:   form.BoardType,
		Type:        projectType,
	}); err != nil {
		ctx.ServerError("NewProject", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.projects.create_success", form.Title))
	ctx.Redirect(setting.AppSubURL + "/")
}
