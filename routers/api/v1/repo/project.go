// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	project_service "code.gitea.io/gitea/services/projects"
)

// ListProjects lists all projects in a repository
func ListProjects(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects repository repoListProjects
	// ---
	// summary: List projects in a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: state
	//   in: query
	//   description: State of the project (open, closed)
	//   type: string
	//   enum: [open, closed, all]
	//   default: open
	// - name: page
	//   in: query
	//   description: page number of results
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !ctx.Repo.CanRead(unit.TypeProjects) {
		ctx.APIErrorNotFound()
		return
	}

	state := ctx.FormTrim("state")
	var isClosed optional.Option[bool]
	switch state {
	case "closed":
		isClosed = optional.Some(true)
	case "open":
		isClosed = optional.Some(false)
	case "all":
		isClosed = optional.None[bool]()
	default:
		isClosed = optional.Some(false)
	}

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	limit := ctx.FormInt("limit")
	if limit <= 0 {
		limit = setting.UI.IssuePagingNum
	}

	projects, count, err := db.FindAndCount[project_model.Project](ctx, project_model.SearchOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: limit,
		},
		RepoID:   ctx.Repo.Repository.ID,
		IsClosed: isClosed,
		Type:     project_model.TypeRepository,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if err := project_service.LoadIssueNumbersForProjects(ctx, projects, ctx.Doer); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiProjects := convert.ToProjectList(ctx, projects)

	ctx.SetLinkHeader(int(count), limit)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiProjects)
}

// GetProject gets a single project
func GetProject(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects/{id} repository repoGetProject
	// ---
	// summary: Get a single project
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Project"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !ctx.Repo.CanRead(unit.TypeProjects) {
		ctx.APIErrorNotFound()
		return
	}

	project, err := project_model.GetProjectForRepoByID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if err := project_service.LoadIssueNumbersForProjects(ctx, []*project_model.Project{project}, ctx.Doer); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToProject(ctx, project))
}

// CreateProject creates a new project
func CreateProject(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects repository repoCreateProject
	// ---
	// summary: Create a new project
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateProjectOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Project"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	if !ctx.Repo.CanWrite(unit.TypeProjects) {
		ctx.APIError(http.StatusForbidden, "no permission")
		return
	}

	form := web.GetForm(ctx).(*api.CreateProjectOption)

	p := &project_model.Project{
		RepoID:       ctx.Repo.Repository.ID,
		Title:        form.Title,
		Description:  form.Description,
		CreatorID:    ctx.Doer.ID,
		TemplateType: project_model.TemplateType(form.TemplateType),
		CardType:     project_model.CardType(form.CardType),
		Type:         project_model.TypeRepository,
	}

	if err := project_model.NewProject(ctx, p); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if err := project_service.LoadIssueNumbersForProjects(ctx, []*project_model.Project{p}, ctx.Doer); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToProject(ctx, p))
}

// EditProject updates a project
func EditProject(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/projects/{id} repository repoEditProject
	// ---
	// summary: Edit a project
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditProjectOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Project"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	if !ctx.Repo.CanWrite(unit.TypeProjects) {
		ctx.APIError(http.StatusForbidden, "no permission")
		return
	}

	project, err := project_model.GetProjectForRepoByID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	form := web.GetForm(ctx).(*api.EditProjectOption)

	if form.Title != nil {
		project.Title = *form.Title
	}
	if form.Description != nil {
		project.Description = *form.Description
	}
	if form.CardType != nil {
		project.CardType = project_model.CardType(*form.CardType)
	}
	if form.IsClosed != nil {
		if err := project_model.ChangeProjectStatus(ctx, project, *form.IsClosed); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	} else {
		if err := project_model.UpdateProject(ctx, project); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	if err := project_service.LoadIssueNumbersForProjects(ctx, []*project_model.Project{project}, ctx.Doer); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToProject(ctx, project))
}

// DeleteProject deletes a project
func DeleteProject(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/projects/{id} repository repoDeleteProject
	// ---
	// summary: Delete a project
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !ctx.Repo.CanWrite(unit.TypeProjects) {
		ctx.APIError(http.StatusForbidden, "no permission")
		return
	}

	if err := project_model.DeleteProjectByID(ctx, ctx.PathParamInt64("id")); err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// ListProjectColumns lists all columns in a project
func ListProjectColumns(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects/{id}/columns repository repoListProjectColumns
	// ---
	// summary: List columns in a project
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectColumnList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !ctx.Repo.CanRead(unit.TypeProjects) {
		ctx.APIErrorNotFound()
		return
	}

	project, err := project_model.GetProjectForRepoByID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	columns, err := project.GetColumns(ctx)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToProjectColumnList(ctx, columns))
}

// CreateProjectColumn creates a new column in a project
func CreateProjectColumn(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects/{id}/columns repository repoCreateProjectColumn
	// ---
	// summary: Create a new column in a project
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateProjectColumnOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/ProjectColumn"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	if !ctx.Repo.CanWrite(unit.TypeProjects) {
		ctx.APIError(http.StatusForbidden, "no permission")
		return
	}

	project, err := project_model.GetProjectForRepoByID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	form := web.GetForm(ctx).(*api.CreateProjectColumnOption)

	column := &project_model.Column{
		Title:     form.Title,
		Color:     form.Color,
		ProjectID: project.ID,
		CreatorID: ctx.Doer.ID,
	}

	if err := project_model.NewColumn(ctx, column); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToProjectColumn(ctx, column))
}

// EditProjectColumn updates a column
func EditProjectColumn(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/projects/columns/{id} repository repoEditProjectColumn
	// ---
	// summary: Edit a project column
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditProjectColumnOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectColumn"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	if !ctx.Repo.CanWrite(unit.TypeProjects) {
		ctx.APIError(http.StatusForbidden, "no permission")
		return
	}

	column, err := project_model.GetColumn(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if project_model.IsErrProjectColumnNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	// Verify column belongs to this repo's project
	_, err = project_model.GetProjectForRepoByID(ctx, ctx.Repo.Repository.ID, column.ProjectID)
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	form := web.GetForm(ctx).(*api.EditProjectColumnOption)

	if form.Title != nil {
		column.Title = *form.Title
	}
	if form.Color != nil {
		column.Color = *form.Color
	}
	if form.Sorting != nil {
		column.Sorting = int8(*form.Sorting)
	}

	if err := project_model.UpdateColumn(ctx, column); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToProjectColumn(ctx, column))
}

// DeleteProjectColumn deletes a column
func DeleteProjectColumn(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/projects/columns/{id} repository repoDeleteProjectColumn
	// ---
	// summary: Delete a project column
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !ctx.Repo.CanWrite(unit.TypeProjects) {
		ctx.APIError(http.StatusForbidden, "no permission")
		return
	}

	column, err := project_model.GetColumn(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if project_model.IsErrProjectColumnNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	// Verify column belongs to this repo's project
	_, err = project_model.GetProjectForRepoByID(ctx, ctx.Repo.Repository.ID, column.ProjectID)
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if err := project_model.DeleteColumnByID(ctx, column.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// AddIssueToProjectColumn adds an issue to a project column
func AddIssueToProjectColumn(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects/columns/{id}/issues repository repoAddIssueToProjectColumn
	// ---
	// summary: Add an issue to a project column
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     type: object
	//     required:
	//       - issue_id
	//     properties:
	//       issue_id:
	//         type: integer
	//         format: int64
	//         description: ID of the issue to add
	// responses:
	//   "201":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	if !ctx.Repo.CanWrite(unit.TypeProjects) {
		ctx.APIError(http.StatusForbidden, "no permission")
		return
	}

	column, err := project_model.GetColumn(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if project_model.IsErrProjectColumnNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	// Verify column belongs to this repo's project
	_, err = project_model.GetProjectForRepoByID(ctx, ctx.Repo.Repository.ID, column.ProjectID)
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	// Parse request body
	form := web.GetForm(ctx).(*api.AddIssueToProjectColumnOption)

	// Add issue to column
	if err := project_model.AddIssueToColumn(ctx, form.IssueID, column); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusCreated)
}
