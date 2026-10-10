// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"net/http"

	project_model "gitea.dev/models/project"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	project_service "gitea.dev/services/projects"
)

func findWorkflow(ctx *context.APIContext, project *project_model.Project) *project_model.Workflow {
	workflow, err := project_model.GetWorkflowByProjectAndID(ctx, project.ID, ctx.PathParamInt64("workflow_id"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return nil
	}
	return workflow
}

func ListProjectWorkflows(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects/{id}/workflows repository repoListProjectWorkflows
	// ---
	// summary: List a project's workflows
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
	//     "$ref": "#/responses/ProjectWorkflowList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation GET /orgs/{org}/projects/{id}/workflows organization orgListProjectWorkflows
	// ---
	// summary: List a project's workflows
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	//     "$ref": "#/responses/ProjectWorkflowList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation GET /user/projects/{id}/workflows user userCurrentListProjectWorkflows
	// ---
	// summary: List a project's workflows
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectWorkflowList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	project := projectScopeFromContext(ctx).findProject(ctx)
	if ctx.Written() {
		return
	}
	workflows, err := project_model.FindWorkflowsByProjectID(ctx, project.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToProjectWorkflowList(workflows))
}

func GetProjectWorkflow(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects/{id}/workflows/{workflow_id} repository repoGetProjectWorkflow
	// ---
	// summary: Get a project workflow
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
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectWorkflow"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation GET /orgs/{org}/projects/{id}/workflows/{workflow_id} organization orgGetProjectWorkflow
	// ---
	// summary: Get a project workflow
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectWorkflow"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation GET /user/projects/{id}/workflows/{workflow_id} user userCurrentGetProjectWorkflow
	// ---
	// summary: Get a project workflow
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectWorkflow"
	//   "404":
	//     "$ref": "#/responses/notFound"

	project := projectScopeFromContext(ctx).findProject(ctx)
	if ctx.Written() {
		return
	}
	workflow := findWorkflow(ctx, project)
	if ctx.Written() {
		return
	}
	ctx.JSON(http.StatusOK, convert.ToProjectWorkflow(workflow))
}

func CreateProjectWorkflow(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects/{id}/workflows repository repoCreateProjectWorkflow
	// ---
	// summary: Create a project workflow
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
	//     "$ref": "#/definitions/CreateProjectWorkflowOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/ProjectWorkflow"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	// swagger:operation POST /orgs/{org}/projects/{id}/workflows organization orgCreateProjectWorkflow
	// ---
	// summary: Create a project workflow
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	//     "$ref": "#/definitions/CreateProjectWorkflowOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/ProjectWorkflow"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	// swagger:operation POST /user/projects/{id}/workflows user userCurrentCreateProjectWorkflow
	// ---
	// summary: Create a project workflow
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateProjectWorkflowOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/ProjectWorkflow"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	project := projectScopeFromContext(ctx).findOpenProject(ctx)
	if ctx.Written() {
		return
	}
	workflow, err := project_service.CreateWorkflow(ctx, ctx.Doer, project, web.GetForm[*api.CreateProjectWorkflowOption](ctx))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToProjectWorkflow(workflow))
}

func EditProjectWorkflow(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/projects/{id}/workflows/{workflow_id} repository repoEditProjectWorkflow
	// ---
	// summary: Edit a project workflow
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
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditProjectWorkflowOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectWorkflow"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	// swagger:operation PATCH /orgs/{org}/projects/{id}/workflows/{workflow_id} organization orgEditProjectWorkflow
	// ---
	// summary: Edit a project workflow
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditProjectWorkflowOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectWorkflow"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	// swagger:operation PATCH /user/projects/{id}/workflows/{workflow_id} user userCurrentEditProjectWorkflow
	// ---
	// summary: Edit a project workflow
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditProjectWorkflowOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectWorkflow"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	project := projectScopeFromContext(ctx).findOpenProject(ctx)
	if ctx.Written() {
		return
	}
	workflow := findWorkflow(ctx, project)
	if ctx.Written() {
		return
	}
	if err := project_service.UpdateWorkflow(ctx, ctx.Doer, project, workflow, web.GetForm[*api.EditProjectWorkflowOption](ctx)); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToProjectWorkflow(workflow))
}

func DeleteProjectWorkflow(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/projects/{id}/workflows/{workflow_id} repository repoDeleteProjectWorkflow
	// ---
	// summary: Delete a project workflow
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
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
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
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	// swagger:operation DELETE /orgs/{org}/projects/{id}/workflows/{workflow_id} organization orgDeleteProjectWorkflow
	// ---
	// summary: Delete a project workflow
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
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

	// swagger:operation DELETE /user/projects/{id}/workflows/{workflow_id} user userCurrentDeleteProjectWorkflow
	// ---
	// summary: Delete a project workflow
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: workflow_id
	//   in: path
	//   description: id of the workflow
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

	project := projectScopeFromContext(ctx).findOpenProject(ctx)
	if ctx.Written() {
		return
	}
	workflow := findWorkflow(ctx, project)
	if ctx.Written() {
		return
	}
	if err := project_model.DeleteWorkflow(ctx, project.ID, workflow.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
