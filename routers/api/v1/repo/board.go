// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/modules/context"
)

func GetProjectBoard(ctx *context.APIContext) {
	// swagger:operation GET /projects/boards/{id} board boardGetProjectBoard
	// ---
	// summary: Get project board
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the board
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectBoard"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
}

func UpdateProjectBoard(ctx *context.APIContext) {
	// swagger:operation PATCH /projects/boards/{id} board boardUpdateProjectBoard
	// ---
	// summary: Update project board
	// produces:
	// - application/json
	// consumes:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project board
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectBoard"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
}

func DeleteProjectBoard(ctx *context.APIContext) {
	// swagger:operation DELETE /projects/boards/{id} board boardDeleteProjectBoard
	// ---
	// summary: Delete project board
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project board
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "description": "Project board deleted"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
}

func ListProjectBoards(ctx *context.APIContext) {
	// swagger:operation GET /projects/{projectId}/boards board boardGetProjectBoards
	// ---
	// summary: Get project boards
	// produces:
	// - application/json
	// parameters:
	// - name: projectId
	//   in: path
	//   description: projectId of the project
	//   type: string
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
	//     "$ref": "#/responses/ProjectBoardList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
}

func CreateProjectBoard(ctx *context.APIContext) {
	// swagger:operation POST /projects/{projectId}/boards board boardCreateProjectBoard
	// ---
	// summary: Create project board
	// produces:
	// - application/json
	// consumes:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: string
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/ProjectBoard"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
}
