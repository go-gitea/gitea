// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"encoding/json"
	"errors"
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	project_service "code.gitea.io/gitea/services/projects"
)

// GetProjectColumn returns a project column
func GetProjectColumn(ctx *context.APIContext) {
	// swagger:operation GET /projects/columns/{column_id} project projectGetProjectColumn
	// ---
	// summary: Get a project column
	// produces:
	// - application/json
	// parameters:
	// - name: column_id
	//   in: path
	//   description: column ID
	//   required: true
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/Column"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	column, err := project_model.GetColumn(ctx, ctx.PathParamInt64(":column_id"))

	if err != nil {
		ctx.NotFoundOrServerError("GetProjectColumn", project_model.IsErrProjectColumnNotExist, err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToColumn(ctx, column))
}

// GetProjectColumns returns a list of project columns
func GetProjectColumns(ctx *context.APIContext) {
	// swagger:operation GET /projects/{project_id}/columns project projectGetProjectColumns
	// ---
	// summary: Get a list of project columns
	// produces:
	// - application/json
	// parameters:
	// - name: project_id
	//   in: path
	//   description: project ID
	//   required: true
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/ColumnList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":project_id"))

	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}

	columns, err := project.GetColumns(ctx)

	if err != nil {
		ctx.ServerError("GetColumnsByProjectID", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToColumns(ctx, columns))
}

// AddColumnToProject adds a new column to a project
func AddColumnToProject(ctx *context.APIContext) {
	// swagger:operation POST /projects/{project_id}/columns project projectAddColumnToProject
	// ---
	// summary: Add a column to a project
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: project_id
	//   in: path
	//   description: project ID
	//   required: true
	//   type: integer
	// - name: body
	//   in: body
	//   description: column data
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreateProjectColumnOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Column"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "412":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	var project *project_model.Project

	projectID := ctx.PathParamInt64(":project_id")

	project, err := project_model.GetProjectByID(ctx, projectID)

	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}

	form := web.GetForm(ctx).(*api.CreateProjectColumnOption)
	column := &project_model.Column{
		ProjectID: project.ID,
		Title:     form.Title,
		Sorting:   form.Sorting,
		Color:     form.Color,
		CreatorID: ctx.Doer.ID,
	}
	if err := project_model.NewColumn(ctx, column); err != nil {
		ctx.ServerError("NewProjectColumn", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToColumn(ctx, column))
}

// EditProjectColumn edits a project column
func EditProjectColumn(ctx *context.APIContext) {
	// swagger:operation PATCH /projects/columns/{column_id} project projectEditProjectColumn
	// ---
	// summary: Edit a project column
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: column_id
	//   in: path
	//   description: column ID
	//   required: true
	//   type: integer
	// - name: body
	//   in: body
	//   description: column data
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/EditProjectColumnOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Column"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "412":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	form := web.GetForm(ctx).(*api.EditProjectColumnOption)
	column, err := project_model.GetColumn(ctx, ctx.PathParamInt64(":column_id"))

	if err != nil {
		ctx.NotFoundOrServerError("GetProjectColumn", project_model.IsErrProjectColumnNotExist, err)
		return
	}

	if form.Title != "" {
		column.Title = form.Title
	}
	column.Color = form.Color
	if form.Sorting != 0 {
		column.Sorting = form.Sorting
	}

	if err := project_model.UpdateColumn(ctx, column); err != nil {
		ctx.ServerError("UpdateProjectColumn", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToColumn(ctx, column))
}

// DeleteProjectColumn deletes a project column
func DeleteProjectColumn(ctx *context.APIContext) {
	// swagger:operation DELETE /projects/columns/{column_id} project projectDeleteProjectColumn
	// ---
	// summary: Delete a project column
	// parameters:
	// - name: column_id
	//   in: path
	//   description: column ID
	//   required: true
	//   type: integer
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	if err := project_model.DeleteColumnByID(ctx, ctx.PathParamInt64(":column_id")); err != nil {
		ctx.ServerError("DeleteProjectColumnByID", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// SetDefaultProjectColumn set default column for issues/pulls
func SetDefaultProjectColumn(ctx *context.APIContext) {
	// swagger:operation PUT /projects/columns/{column_id}/default project projectSetDefaultProjectColumn
	// ---
	// summary: Set default column for issues/pulls
	// parameters:
	// - name: column_id
	//   in: path
	//   description: column ID
	//   required: true
	//   type: integer
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	column, err := project_model.GetColumn(ctx, ctx.PathParamInt64(":column_id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectColumn", project_model.IsErrProjectColumnNotExist, err)
		return
	}

	if err := project_model.SetDefaultColumn(ctx, column.ProjectID, column.ID); err != nil {
		ctx.ServerError("SetDefaultColumn", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// MoveColumns moves or keeps columns in a project and sorts them inside that project
func MoveColumns(ctx *context.APIContext) {
	// swagger:operation PATCH /projects/{project_id}/columns/move project projectMoveColumns
	// ---
	// summary: Move columns in a project
	// consumes:
	// - application/json
	// parameters:
	// - name: project_id
	//   in: path
	//   description: project ID
	//   required: true
	//   type: integer
	// - name: body
	//   in: body
	//   description: columns data
	//   required: true
	//   schema:
	//    "$ref": "#/definitions/MovedColumnsOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/ColumnList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":project_id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}

	form := &api.MovedColumnsOption{}
	if err = json.NewDecoder(ctx.Req.Body).Decode(&form); err != nil {
		ctx.ServerError("DecodeMovedColumnsForm", err)
		return
	}

	sortedColumnIDs := make(map[int64]int64)
	for _, column := range form.Columns {
		sortedColumnIDs[column.Sorting] = column.ColumnID
	}

	if err = project_model.MoveColumnsOnProject(ctx, project, sortedColumnIDs); err != nil {
		ctx.ServerError("MoveColumnsOnProject", err)
		return
	}

	columns, err := project.GetColumns(ctx)

	if err != nil {
		ctx.ServerError("GetColumns", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToColumns(ctx, columns))
}

// MoveIssues moves or keeps issues in a column and sorts them inside that column
func MoveIssues(ctx *context.APIContext) {
	// swagger:operation PATCH /projects/{project_id}/columns/{column_id}/move project projectMoveIssues
	// ---
	// summary: Move issues in a column
	// consumes:
	// - application/json
	// parameters:
	// - name: project_id
	//   in: path
	//   description: project ID
	//   required: true
	//   type: integer
	// - name: column_id
	//   in: path
	//   description: column ID
	//   required: true
	//   type: integer
	// - name: body
	//   in: body
	//   description: issues data
	//   required: true
	//   schema:
	//    "$ref": "#/definitions/MovedIssuesOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/IssueList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":project_id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}

	column, err := project_model.GetColumn(ctx, ctx.PathParamInt64(":column_id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectColumn", project_model.IsErrProjectColumnNotExist, err)
		return
	}

	form := &api.MovedIssuesOption{}
	if err = json.NewDecoder(ctx.Req.Body).Decode(&form); err != nil {
		ctx.ServerError("DecodeMovedIssuesForm", err)
		return
	}

	issueIDs := make([]int64, 0, len(form.Issues))
	sortedIssueIDs := make(map[int64]int64)
	for _, issue := range form.Issues {
		issueIDs = append(issueIDs, issue.IssueID)
		sortedIssueIDs[issue.Sorting] = issue.IssueID
	}
	movedIssues, err := issues_model.GetIssuesByIDs(ctx, issueIDs)
	if err != nil {
		ctx.NotFoundOrServerError("GetIssueByID", issues_model.IsErrIssueNotExist, err)
		return
	}

	if len(movedIssues) != len(form.Issues) {
		ctx.ServerError("some issues do not exist", errors.New("some issues do not exist"))
		return
	}

	if _, err = movedIssues.LoadRepositories(ctx); err != nil {
		ctx.ServerError("LoadRepositories", err)
		return
	}

	for _, issue := range movedIssues {
		if issue.RepoID != project.RepoID && issue.Repo.OwnerID != project.OwnerID {
			ctx.ServerError("Some issue's repoID is not equal to project's repoID", errors.New("some issue's repoID is not equal to project's repoID"))
			return
		}
	}

	if err = project_service.MoveIssuesOnProjectColumn(ctx, ctx.Doer, column, sortedIssueIDs); err != nil {
		ctx.ServerError("MoveIssuesOnProjectColumn", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToAPIIssueList(ctx, ctx.Doer, movedIssues))
}
