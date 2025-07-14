// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

func innerCreateProject(ctx *context.APIContext, projectType project_model.Type) {
	form := web.GetForm(ctx).(*api.NewProjectOption)
	project := &project_model.Project{
		Title:        form.Name,
		Description:  form.Body,
		CreatorID:    ctx.Doer.ID,
		TemplateType: project_model.ToTemplateType(form.TemplateType),
		Type:         projectType,
	}

	if ctx.ContextUser == nil {
		ctx.APIError(http.StatusForbidden, "Not authenticated")
		return
	}
	project.OwnerID = ctx.ContextUser.ID

	if projectType == project_model.TypeRepository {
		project.RepoID = ctx.Repo.Repository.ID
	}

	if err := project_model.NewProject(ctx, project); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	project, err := project_model.GetProjectByID(ctx, project.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	projectResponse, err := convert.ToAPIProject(ctx, project)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusCreated, projectResponse)
}

func CreateUserProject(ctx *context.APIContext) {
	// swagger:operation POST /user/projects project projectCreateUserProject
	// ---
	// summary: Create a user project
	// produces:
	// - application/json
	// consumes:
	// - application/json
	// parameters:
	//   - name: project
	//     in: body
	//     required: true
	//     schema: { "$ref": "#/definitions/NewProjectOption" }
	// responses:
	//  "201":
	//    "$ref": "#/responses/Project"
	//  "403":
	//    "$ref": "#/responses/forbidden"
	//  "404":
	//    "$ref": "#/responses/notFound"
	innerCreateProject(ctx, project_model.TypeIndividual)
}

func CreateOrgProject(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/projects project projectCreateOrgProject
	// ---
	// summary: Create a organization project
	// produces:
	// - application/json
	// consumes:
	// - application/json
	// parameters:
	//   - name: org
	//     in: path
	//     description: owner of repo
	//     type: string
	//     required: true
	//   - name: project
	//     in: body
	//     required: true
	//     schema: { "$ref": "#/definitions/NewProjectOption" }
	// responses:
	//  "201":
	//    "$ref": "#/responses/Project"
	//  "403":
	//    "$ref": "#/responses/forbidden"
	// "404":
	//    "$ref": "#/responses/notFound"
	innerCreateProject(ctx, project_model.TypeOrganization)
}

func CreateRepoProject(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects project projectCreateRepositoryProject
	// ---
	// summary: Create a repository project
	// produces:
	// - application/json
	// consumes:
	// - application/json
	// parameters:
	//   - name: owner
	//     in: path
	//     description: owner of repo
	//     type: string
	//     required: true
	//   - name: repo
	//     in: path
	//     description: repo
	//     type: string
	//     required: true
	//   - name: project
	//     in: body
	//     required: true
	//     schema: { "$ref": "#/definitions/NewProjectOption" }
	// responses:
	//  "201":
	//    "$ref": "#/responses/Project"
	//  "403":
	//    "$ref": "#/responses/forbidden"
	//  "404":
	//    "$ref": "#/responses/notFound"
	innerCreateProject(ctx, project_model.TypeRepository)
}

func GetProject(ctx *context.APIContext) {
	// swagger:operation GET /projects/{project_id} project projectGetProject
	// ---
	// summary: Get project
	// produces:
	// - application/json
	// parameters:
	//   - name: id
	//     in: path
	//     description: id of the project
	//     type: string
	//     required: true
	// responses:
	//  "200":
	//    "$ref": "#/responses/Project"
	//  "403":
	//    "$ref": "#/responses/forbidden"
	//  "404":
	//    "$ref": "#/responses/notFound"
	project, err := project_model.GetProjectByID(ctx, ctx.FormInt64("project_id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	projectResponse, err := convert.ToAPIProject(ctx, project)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, projectResponse)
}

func UpdateProject(ctx *context.APIContext) {
	// swagger:operation PATCH /projects/{project_id} project projectUpdateProject
	// ---
	// summary: Update project
	// produces:
	// - application/json
	// consumes:
	// - application/json
	// parameters:
	//   - name: id
	//     in: path
	//     description: id of the project
	//     type: string
	//     required: true
	//   - name: project
	//     in: body
	//     required: true
	//     schema: { "$ref": "#/definitions/UpdateProjectOption" }
	// responses:
	//  "200":
	//    "$ref": "#/responses/Project"
	//  "403":
	//    "$ref": "#/responses/forbidden"
	//  "404":
	//    "$ref": "#/responses/notFound"
	form := web.GetForm(ctx).(*api.UpdateProjectOption)
	project, err := project_model.GetProjectByID(ctx, ctx.FormInt64("project_id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	if project.Title != form.Name {
		project.Title = form.Name
	}
	if project.Description != form.Body {
		project.Description = form.Body
	}

	err = project_model.UpdateProject(ctx, project)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	projectResponse, err := convert.ToAPIProject(ctx, project)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, projectResponse)
}

func DeleteProject(ctx *context.APIContext) {
	// swagger:operation DELETE /projects/{project_id} project projectDeleteProject
	// ---
	// summary: Delete project
	// parameters:
	//   - name: id
	//     in: path
	//     description: id of the project
	//     type: string
	//     required: true
	// responses:
	//  "204":
	//    "description": "Deleted the project"
	//  "403":
	//    "$ref": "#/responses/forbidden"
	//  "404":
	//    "$ref": "#/responses/notFound"

	if err := project_model.DeleteProjectByID(ctx, ctx.FormInt64("project_id")); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func ListUserProjects(ctx *context.APIContext) {
	// swagger:operation GET /users/{user}/projects project projectListUserProjects
	// ---
	// summary: List user projects
	// produces:
	// - application/json
	// parameters:
	//   - name: closed
	//     in: query
	//     description: include closed projects or not
	//     type: boolean
	//   - name: page
	//     in: query
	//     description: page number of results to return (1-based)
	//     type: integer
	//   - name: limit
	//     in: query
	//     description: page size of results
	//     type: integer
	// responses:
	//  "200":
	//    "$ref": "#/responses/ProjectList"
	//  "403":
	//    "$ref": "#/responses/forbidden"
	//  "404":
	//    "$ref": "#/responses/notFound"

	listOptions := utils.GetListOptions(ctx)
	projects, count, err := db.FindAndCount[project_model.Project](ctx, project_model.SearchOptions{
		Type:        project_model.TypeIndividual,
		IsClosed:    ctx.FormOptionalBool("closed"),
		OwnerID:     ctx.Doer.ID,
		ListOptions: listOptions,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetLinkHeader(int(count), listOptions.PageSize)
	ctx.SetTotalCountHeader(count)

	apiProjects, err := convert.ToAPIProjectList(ctx, projects)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, apiProjects)
}

func ListOrgProjects(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/projects project projectListOrgProjects
	// ---
	// summary: List org projects
	// produces:
	// - application/json
	// parameters:
	//   - name: org
	//     in: path
	//     description: owner of the repository
	//     type: string
	//     required: true
	//   - name: closed
	//     in: query
	//     description: include closed projects or not
	//     type: boolean
	//   - name: page
	//     in: query
	//     description: page number of results to return (1-based)
	//     type: integer
	//   - name: limit
	//     in: query
	//     description: page size of results
	//     type: integer
	// responses:
	//  "200":
	//    "$ref": "#/responses/ProjectList"
	//  "403":
	//    "$ref": "#/responses/forbidden"
	//  "404":
	//    "$ref": "#/responses/notFound"

	listOptions := utils.GetListOptions(ctx)
	projects, count, err := db.FindAndCount[project_model.Project](ctx, project_model.SearchOptions{
		OwnerID:     ctx.Org.Organization.AsUser().ID,
		ListOptions: listOptions,
		IsClosed:    ctx.FormOptionalBool("closed"),
		Type:        project_model.TypeOrganization,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetLinkHeader(int(count), listOptions.PageSize)
	ctx.SetTotalCountHeader(count)

	apiProjects, err := convert.ToAPIProjectList(ctx, projects)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, apiProjects)
}

func ListRepoProjects(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects project projectListRepositoryProjects
	// ---
	// summary: List repository projects
	// produces:
	// - application/json
	// parameters:
	//   - name: owner
	//     in: path
	//     description: owner of the repository
	//     type: string
	//     required: true
	//   - name: repo
	//     in: path
	//     description: repo
	//     type: string
	//     required: true
	//   - name: closed
	//     in: query
	//     description: include closed projects or not
	//     type: boolean
	//   - name: page
	//     in: query
	//     description: page number of results to return (1-based)
	//     type: integer
	//   - name: limit
	//     in: query
	//     description: page size of results
	//     type: integer
	// responses:
	//  "200":
	//    "$ref": "#/responses/ProjectList"
	//  "403":
	//    "$ref": "#/responses/forbidden"
	//  "404":
	//    "$ref": "#/responses/notFound"

	listOptions := utils.GetListOptions(ctx)
	projects, count, err := db.FindAndCount[project_model.Project](ctx, project_model.SearchOptions{
		RepoID:      ctx.Repo.Repository.ID,
		IsClosed:    ctx.FormOptionalBool("closed"),
		Type:        project_model.TypeRepository,
		ListOptions: listOptions,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetLinkHeader(int(count), listOptions.PageSize)
	ctx.SetTotalCountHeader(count)

	apiProjects, err := convert.ToAPIProjectList(ctx, projects)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, apiProjects)
}
