// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/modules/context"
)

// CreateProject create a project for a repository
func CreateProject(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects repository repoCreateProject
	// ---
	// summary: Create a project for a repository
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
	//     "$ref": "#/definitions/UpsertProjectPayload"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Project"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "412":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"
}

// EditProject
func EditProject(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/projects/{id} repository repoEditProject
	// ---
	// summary: Edit a project
	// produces:
	// - application/json
	// consumes:
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
	//   type: string
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/Project"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "412":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

}

// GetProject a single project for repository
func GetProject(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects/{id} repository repoGetProject
	// ---
	// summary: List a single project
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
	//   type: string
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/Project"
	//   "404":
	//     "$ref": "#/responses/notFound"
}

// ListProjects list all the projects for a particular repository
func ListProjects(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects repository repoListProjects
	// ---
	// summary: List a repository's projects
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
	// responses:
	//   "201":
	//     "$ref": "#/responses/ProjectList"
}

// DeleteProject delete a project from particular repository
func DeleteProject(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/projects/{id} repository repoDeleteProject
	// ---
	// summary: Delete a project
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
	//   type: string
	//   required: true
	// responses:
	//   "404":
	//     "$ref": "#/responses/notFound"
}

// Project Boards

func CreateBoard(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects/{projectId}/boards repository repoCreateProjectBoard
	// ---
	// summary: Create a board
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
	// - name: projectId
	//   in: path
	//   description: project id
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UpsertProjectBoardPayload"
	// responses:
	//   "201":
	//     "$ref": "#/responses/ProjectBoard"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "412":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"
}

func EditProjectBoard(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/projects/{projectId}/boards/{id} repository repoEditProjectBoard
	// ---
	// summary: Edit Project Board
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
	// - name: projectId
	//   in: path
	//   description: project id
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: board id
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UpsertProjectBoardPayload"
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectBoard"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "412":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"
}

func GetProjectBoard(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects/{projectId}/boards/{id} repository repoGetProjectBoard
	// ---
	// summary: Create a board
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
	// - name: projectId
	//   in: path
	//   description: project id
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: project id
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectBoard"
	//   "403":
	//     "$ref": "#/responses/forbidden"
}

func ListProjectBoards(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects/{projectId}/boards repository repoListProjectBoards
	// ---
	// summary: Get list of project boards
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
	// - name: projectId
	//   in: path
	//   description: project id
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectBoardList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
}

func DeleteProjectBoard(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/projects/{projectId}/boards/{id} repository repoDeleteProjectBoard
	// ---
	// summary: Delete project board
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
	// - name: projectId
	//   in: path
	//   description: project id
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: board id
	//   type: string
	//   required: true
	// responses:
	//   "403":
	//     "$ref": "#/responses/forbidden"
}
