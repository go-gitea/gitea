// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/optional"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	project_service "code.gitea.io/gitea/services/projects"
)

func getRepoProjectByID(ctx *context.APIContext) *project_model.Project {
	project, err := project_model.GetProjectForRepoByID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil
	}
	project.Repo = ctx.Repo.Repository
	return project
}

func getRepoProjectColumn(ctx *context.APIContext) *project_model.Column {
	column, err := project_model.GetColumn(ctx, ctx.PathParamInt64("column_id"))
	if err != nil {
		if project_model.IsErrProjectColumnNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil
	}
	p, err := project_model.GetProjectForRepoByID(ctx, ctx.Repo.Repository.ID, column.ProjectID)
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil
	}
	if p.ID != ctx.PathParamInt64("id") {
		ctx.APIErrorNotFound()
		return nil
	}

	return column
}

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

	isClosed := common.ParseIssueFilterStateIsClosed(ctx.FormTrim("state"))

	listOptions := utils.GetListOptions(ctx)

	projects, count, err := db.FindAndCount[project_model.Project](ctx, project_model.SearchOptions{
		ListOptions: listOptions,
		RepoID:      ctx.Repo.Repository.ID,
		IsClosed:    isClosed,
		Type:        project_model.TypeRepository,
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

	ctx.SetLinkHeader(count, listOptions.PageSize)
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

	project := getRepoProjectByID(ctx)
	if ctx.Written() {
		return
	}

	if err := project_service.LoadIssueNumbersForProject(ctx, project, ctx.Doer); err != nil {
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

	project := getRepoProjectByID(ctx)
	if ctx.Written() {
		return
	}

	form := web.GetForm(ctx).(*api.EditProjectOption)

	opts := project_service.UpdateProjectOptions{
		Title:       optional.FromPtr(form.Title),
		Description: optional.FromPtr(form.Description),
	}
	if form.CardType != nil {
		opts.CardType = optional.Some(project_model.CardType(*form.CardType))
	}
	if form.State != nil {
		opts.IsClosed = optional.Some(*form.State == string(api.StateClosed))
	}
	if err := project_service.UpdateProject(ctx, project, opts); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if err := project_service.LoadIssueNumbersForProject(ctx, project, ctx.Doer); err != nil {
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

	project := getRepoProjectByID(ctx)
	if ctx.Written() {
		return
	}

	if err := project_model.DeleteProjectByID(ctx, project.ID); err != nil {
		ctx.APIErrorInternal(err)
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
	//     "$ref": "#/responses/ProjectColumnList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	project := getRepoProjectByID(ctx)
	if ctx.Written() {
		return
	}

	total, err := project_model.CountProjectColumns(ctx, project.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	listOptions := utils.GetListOptions(ctx)
	columns, err := project_model.GetProjectColumns(ctx, project.ID, listOptions)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetLinkHeader(total, listOptions.PageSize)
	ctx.SetTotalCountHeader(total)
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

	project := getRepoProjectByID(ctx)
	if ctx.Written() {
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
	// swagger:operation PATCH /repos/{owner}/{repo}/projects/{id}/columns/{column_id} repository repoEditProjectColumn
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
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
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

	column := getRepoProjectColumn(ctx)
	if ctx.Written() {
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
		column.Sorting = *form.Sorting
	}

	if err := project_model.UpdateColumn(ctx, column); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToProjectColumn(ctx, column))
}

// DeleteProjectColumn deletes a column
func DeleteProjectColumn(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/projects/{id}/columns/{column_id} repository repoDeleteProjectColumn
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
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
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

	column := getRepoProjectColumn(ctx)
	if ctx.Written() {
		return
	}

	if err := project_model.DeleteColumnByID(ctx, column.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// ListProjectColumnIssues lists all issues in a project column
func ListProjectColumnIssues(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects/{id}/columns/{column_id}/issues repository repoListProjectColumnIssues
	// ---
	// summary: List issues in a project column
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
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
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
	//     "$ref": "#/responses/IssueList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	column := getRepoProjectColumn(ctx)
	if ctx.Written() {
		return
	}

	listOptions := utils.GetListOptions(ctx)
	issuesOpts := &issues_model.IssuesOptions{
		Paginator:       &listOptions,
		RepoIDs:         []int64{ctx.Repo.Repository.ID},
		ProjectID:       column.ProjectID,
		ProjectColumnID: column.ID,
		SortType:        issues_model.SortTypeProjectColumnSorting,
	}

	count, err := issues_model.CountIssues(ctx, issuesOpts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	issues, err := issues_model.Issues(ctx, issuesOpts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetLinkHeader(count, listOptions.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, convert.ToAPIIssueList(ctx, ctx.Doer, issues))
}

// AddIssueToProjectColumn adds an issue to a project column
func AddIssueToProjectColumn(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects/{id}/columns/{column_id}/issues/{issue_id} repository repoAddIssueToProjectColumn
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
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// - name: issue_id
	//   in: path
	//   description: id of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	assignIssueToProjectColumn(ctx, true)
}

// RemoveIssueFromProjectColumn remove an issue from a project column
func RemoveIssueFromProjectColumn(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/projects/{id}/columns/{column_id}/issues/{issue_id} repository repoRemoveIssueFromProjectColumn
	// ---
	// summary: Remove an issue from a project column
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
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// - name: issue_id
	//   in: path
	//   description: id of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	assignIssueToProjectColumn(ctx, false)
}

// assignIssueToProjectColumn assigns an issue to a project column when add is true,
// or removes the issue from any project assignment when add is false.
func assignIssueToProjectColumn(ctx *context.APIContext, add bool) {
	column := getRepoProjectColumn(ctx)
	if ctx.Written() {
		return
	}

	issue, err := issues_model.GetIssueByRepoID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("issue_id"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	projectID := int64(0)
	if add {
		projectID = column.ProjectID
	}
	if err := issues_model.IssueAssignOrRemoveProject(ctx, issue, ctx.Doer, projectID, column.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if add {
		ctx.Status(http.StatusCreated)
	} else {
		ctx.Status(http.StatusNoContent)
	}
}
