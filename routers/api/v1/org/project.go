// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/optional"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// CreateProject creates a new project for organization
func CreateProject(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/projects project orgCreateProject
	// ---
	// summary: Create a new project
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: organization name that the project belongs to
	//   required: true
	//   type: string
	// - name: body
	//   in: body
	//   description: Project data
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreateProjectOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Project"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "412":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateProjectOption)

	project := &project_model.Project{
		Title:        form.Title,
		Description:  form.Content,
		CreatorID:    ctx.Doer.ID,
		TemplateType: project_model.TemplateType(form.TemplateType),
		CardType:     project_model.CardType(form.CardType),
		Type:         project_model.TypeOrganization,
		OwnerID:      ctx.ContextUser.ID,
	}

	if err := project_model.NewProject(ctx, project); err != nil {
		ctx.Error(http.StatusInternalServerError, "NewProject", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToProject(ctx, project))
}

// GetProjects returns a list of projects that belong to an organization
func GetProjects(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/projects project orgGetProjects
	// ---
	// summary: Get a list of projects
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: organization name that the project belongs to
	//   required: true
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	listOptions := utils.GetListOptions(ctx)
	sortType := ctx.FormTrim("sort")

	isShowClosed := strings.ToLower(ctx.FormTrim("state")) == "closed"

	searchOptions := project_model.SearchOptions{
		ListOptions: listOptions,
		IsClosed:    optional.Some(isShowClosed),
		OrderBy:     project_model.GetSearchOrderByBySortType(sortType),
		OwnerID:     ctx.ContextUser.ID,
		Type:        project_model.TypeOrganization,
	}

	projects, maxResults, err := db.FindAndCount[project_model.Project](ctx, &searchOptions)

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "db.FindAndCount[project_model.Project]", err)
		return
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, convert.ToProjects(ctx, projects))
}
