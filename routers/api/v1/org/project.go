package org

import (
	"log"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
)

// CreateProject creates a new project
func CreateProject(ctx *context.APIContext) {
	form := web.GetForm(ctx).(*api.CreateProjectOption)

	log.Println(ctx.ContextUser.ID)

	project := &project_model.Project{
		OwnerID:      ctx.ContextUser.ID,
		Title:        form.Title,
		Description:  form.Content,
		CreatorID:    ctx.Doer.ID,
		TemplateType: project_model.TemplateType(form.TemplateType),
		CardType:     project_model.CardType(form.CardType),
	}

	if ctx.ContextUser.IsOrganization() {
		project.Type = project_model.TypeOrganization
	} else {
		project.Type = project_model.TypeIndividual
	}

	if err := project_model.NewProject(ctx, project); err != nil {
		ctx.Error(http.StatusInternalServerError, "NewProject", err)
		return
	}

	ctx.JSON(http.StatusCreated, map[string]int64{"id": project.ID})
}

// ChangeProjectStatus updates the status of a project between "open" and "close"
func ChangeProjectStatus(ctx *context.APIContext) {
	var toClose bool
	switch ctx.PathParam(":action") {
	case "open":
		toClose = false
	case "close":
		toClose = true
	default:
		ctx.NotFound("ChangeProjectStatus", nil)
		return
	}
	id := ctx.PathParamInt64(":id")

	if err := project_model.ChangeProjectStatusByRepoIDAndID(ctx, 0, id, toClose); err != nil {
		ctx.NotFoundOrServerError("ChangeProjectStatusByRepoIDAndID", project_model.IsErrProjectNotExist, err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{"message": "project status updated successfully"})
}

// Projects renders the home page of projects
func GetProjects(ctx *context.APIContext) {
	ctx.Data["Title"] = ctx.Tr("repo.projects")

	sortType := ctx.FormTrim("sort")

	isShowClosed := strings.ToLower(ctx.FormTrim("state")) == "closed"
	keyword := ctx.FormTrim("q")
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	var projectType project_model.Type
	if ctx.ContextUser.IsOrganization() {
		projectType = project_model.TypeOrganization
	} else {
		projectType = project_model.TypeIndividual
	}
	projects, err := db.Find[project_model.Project](ctx, project_model.SearchOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: setting.UI.IssuePagingNum,
		},
		OwnerID:  ctx.ContextUser.ID,
		IsClosed: optional.Some(isShowClosed),
		OrderBy:  project_model.GetSearchOrderByBySortType(sortType),
		Type:     projectType,
		Title:    keyword,
	})
	if err != nil {
		ctx.ServerError("FindProjects", err)
		return
	}

	ctx.JSON(http.StatusOK, projects)
}
