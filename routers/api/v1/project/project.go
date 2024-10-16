// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// GetProject returns a project
func GetProject(ctx *context.APIContext) {
	// swagger:operation GET /projects/{project_id} project projectGetProject
	// ---
	// summary: Get a project
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
	//     "$ref": "#/responses/Project"
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
		ctx.ServerError("GetProjectColumns", err)
		return
	}

	issuesMap, err := issues_model.LoadIssuesFromColumnList(ctx, columns, &issues_model.IssuesOptions{})
	if err != nil {
		ctx.ServerError("LoadIssuesOfColumns", err)
		return
	}

	issues := issues_model.IssueList{}

	for _, column := range columns {
		if empty := issuesMap[column.ID]; len(empty) == 0 {
			continue
		}
		issues = append(issues, issuesMap[column.ID]...)
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"project": convert.ToProject(ctx, project),
		"columns": convert.ToColumns(ctx, columns),
		"issues":  convert.ToAPIIssueList(ctx, ctx.Doer, issues),
	})
}

// EditProject edits a project
func EditProject(ctx *context.APIContext) {
	// swagger:operation PATCH /projects/{project_id} project projectEditProject
	// ---
	// summary: Edit a project
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
	//     "$ref": "#/responses/Project"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "412":
	//     "$ref": "#/responses/error"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	form := web.GetForm(ctx).(*api.EditProjectOption)
	projectID := ctx.PathParamInt64(":project_id")

	project, err := project_model.GetProjectByID(ctx, projectID)
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}

	project.Title = form.Title
	project.Description = form.Content
	project.CardType = project_model.CardType(form.CardType)

	if err = project_model.UpdateProject(ctx, project); err != nil {
		ctx.ServerError("UpdateProjects", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToProject(ctx, project))
}

// DeleteProject deletes a project
func DeleteProject(ctx *context.APIContext) {
	// swagger:operation DELETE /projects/{project_id} project projectDeleteProject
	// ---
	// summary: Delete a project
	// description: Deletes a specific project for a given user and repository.
	// parameters:
	// - name: project_id
	//   in: path
	//   description: project ID
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

	err := project_model.DeleteProjectByID(ctx, ctx.PathParamInt64(":project_id"))

	if err != nil {
		ctx.ServerError("DeleteProjectByID", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// ChangeProjectStatus updates the status of a project between "open" and "close"
func ChangeProjectStatus(ctx *context.APIContext) {
	// swagger:operation PATCH /projects/{project_id}/{action} project projectProjectChangeProjectStatus
	// ---
	// summary: Change the status of a project
	// produces:
	// - application/json
	// parameters:
	// - name: project_id
	//   in: path
	//   description: project ID
	//   required: true
	//   type: integer
	// - name: action
	//   in: path
	//   description: action to perform (open or close)
	//   required: true
	//   type: string
	//   enum:
	//   - open
	//   - close
	// responses:
	//   "200":
	//     "$ref": "#/responses/Project"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

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
	id := ctx.PathParamInt64(":project_id")

	if err := project_model.ChangeProjectStatusByRepoIDAndID(ctx, 0, id, toClose); err != nil {
		ctx.NotFoundOrServerError("ChangeProjectStatusByRepoIDAndID", project_model.IsErrProjectNotExist, err)
		return
	}

	project, err := project_model.GetProjectByID(ctx, id)

	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToProject(ctx, project))
}
