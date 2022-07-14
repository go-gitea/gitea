// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/modules/context"
)

func GetProject(ctx *context.APIContext) {
	// swagger:operation GET /projects/{id} project projectGetProject
	// ---
	// summary: Get project
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Project"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
}

func UpdateProject(ctx *context.APIContext) {
	// swagger:operation PATCH /projects/{id} project projectUpdateProject
	// ---
	// summary: Update project
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Project"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
}

func DeleteProject(ctx *context.APIContext) {
	// swagger:operation DELETE /projects/{id} project projectDeleteProject
	// ---
	// summary: Delete project
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "description": "Deleted the project"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
}

func CreateRepositoryProject(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects project projectCreateRepositoryProject
	// ---
	// summary: Create a repository project
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: repo
	//   type: string
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/Project"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
}

func ListRepositoryProjects(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects project projectListRepositoryProjects
	// ---
	// summary: List repository projects
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repository
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: repo
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
	//     "$ref": "#/responses/ProjectList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
}
