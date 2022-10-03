// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/gitref"
)

// GetGitAllRefs get ref or an list all the refs of a repository
func GetGitAllRefs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/refs repository repoListAllGitRefs
	// ---
	// summary: Get specified ref or filtered repository's refs
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
	//   "200":
	// #   "$ref": "#/responses/Reference" TODO: swagger doesnt support different output formats by ref
	//     "$ref": "#/responses/ReferenceList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	getGitRefsInternal(ctx, "")
}

// GetGitRefs get ref or an filteresd list of refs of a repository
func GetGitRefs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/refs/{ref} repository repoListGitRefs
	// ---
	// summary: Get specified ref or filtered repository's refs
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
	// - name: ref
	//   in: path
	//   description: part or full name of the ref
	//   type: string
	//   required: true
	// responses:
	//   "200":
	// #   "$ref": "#/responses/Reference" TODO: swagger doesnt support different output formats by ref
	//     "$ref": "#/responses/ReferenceList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	getGitRefsInternal(ctx, ctx.Params("*"))
}

func getGitRefsInternal(ctx *context.APIContext, filter string) {
	refs, lastMethodName, err := utils.GetGitRefs(ctx, filter)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, lastMethodName, err)
		return
	}

	if len(refs) == 0 {
		ctx.NotFound()
		return
	}

	apiRefs := make([]*api.Reference, len(refs))
	for i := range refs {
		apiRefs[i] = convert.ToGitRef(ctx.Repo.Repository, refs[i])
	}
	// If single reference is found and it matches filter exactly return it as object
	if len(apiRefs) == 1 && apiRefs[0].Ref == filter {
		ctx.JSON(http.StatusOK, &apiRefs[0])
		return
	}
	ctx.JSON(http.StatusOK, &apiRefs)
}

// CreateGitRef creates a git ref for a repository that points to a target commitish
func CreateGitRef(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/git/refs repository repoCreateGitRef
	// ---
	// summary: Create a reference
	// description: Creates a reference for your repository. You are unable to create new references for empty repositories,
	//             even if the commit SHA-1 hash used exists. Empty repositories are repositories without branches.
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
	//     "$ref": "#/definitions/CreateGitRefOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Reference"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     description: The git ref with the same name already exists.
	//   "422":
	//     description: Unable to form reference

	opt := web.GetForm(ctx).(*api.CreateGitRefOption)

	if ctx.Repo.GitRepo.IsReferenceExist(opt.RefName) {
		ctx.Error(http.StatusConflict, "reference exists", fmt.Errorf("reference already exists: %s", opt.RefName))
		return
	}

	commitID, err := ctx.Repo.GitRepo.GetRefCommitID(opt.Target)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.Error(http.StatusNotFound, "invalid target", fmt.Errorf("target does not exist: %s", opt.Target))
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetRefCommitID", err)
		return
	}

	ref, err := gitref.UpdateReferenceWithChecks(ctx, opt.RefName, commitID)
	if err != nil {
		if git.IsErrInvalidRefName(err) {
			ctx.Error(http.StatusUnprocessableEntity, "invalid reference'", err)
		} else if git.IsErrProtectedRefName(err) {
			ctx.Error(http.StatusMethodNotAllowed, "protected reference", err)
		} else if git.IsErrRefNotFound(err) {
			ctx.Error(http.StatusUnprocessableEntity, "UpdateReferenceWithChecks", fmt.Errorf("unable to load reference [ref_name: %s]", opt.RefName))
		} else {
			ctx.InternalServerError(err)
		}
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToGitRef(ctx.Repo.Repository, ref))
}

// UpdateGitRef updates a branch for a repository from a commit SHA
func UpdateGitRef(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/git/refs/{ref} repository repoUpdateGitRef
	// ---
	// summary: Update a reference
	// description:
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
	// - name: ref
	//   in: path
	//   description: name of the ref to update
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UpdateGitRefOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Reference"
	//   "404":
	//     "$ref": "#/responses/notFound"

	refName := fmt.Sprintf("refs/%s", ctx.Params("*"))
	opt := web.GetForm(ctx).(*api.UpdateGitRefOption)

	if ctx.Repo.GitRepo.IsReferenceExist(refName) {
		ctx.Error(http.StatusConflict, "reference exists", fmt.Errorf("reference already exists: %s", refName))
		return
	}

	commitID, err := ctx.Repo.GitRepo.GetRefCommitID(opt.Target)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.Error(http.StatusNotFound, "invalid target", fmt.Errorf("target does not exist: %s", opt.Target))
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetRefCommitID", err)
		return
	}

	ref, err := gitref.UpdateReferenceWithChecks(ctx, refName, commitID)
	if err != nil {
		if git.IsErrInvalidRefName(err) {
			ctx.Error(http.StatusUnprocessableEntity, "invalid reference'", err)
		} else if git.IsErrProtectedRefName(err) {
			ctx.Error(http.StatusMethodNotAllowed, "protected reference", err)
		} else if git.IsErrRefNotFound(err) {
			ctx.Error(http.StatusUnprocessableEntity, "UpdateReferenceWithChecks", fmt.Errorf("unable to load reference [ref_name: %s]", refName))
		} else {
			ctx.InternalServerError(err)
		}
		return
	}

	ctx.JSON(http.StatusCreated, ref)
}

// DeleteGitRef deletes a git ref for a repository that points to a target commitish
func DeleteGitRef(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/git/refs/{ref} repository repoDeleteGitRef
	// ---
	// summary: Delete a reference
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
	// - name: ref
	//   in: path
	//   description: name of the ref to be deleted
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "405":
	//     "$ref": "#/responses/error"
	//   "409":
	//     "$ref": "#/responses/conflict"

	refName := fmt.Sprintf("refs/%s", ctx.Params("*"))

	if !ctx.Repo.GitRepo.IsReferenceExist(refName) {
		ctx.Error(http.StatusNotFound, "git ref does not exist:", fmt.Errorf("reference does not exist: %s", refName))
		return
	}

	err := gitref.RemoveReferenceWithChecks(ctx, refName)
	if err != nil {
		if git.IsErrInvalidRefName(err) {
			ctx.Error(http.StatusUnprocessableEntity, "invalid reference'", err)
		} else if git.IsErrProtectedRefName(err) {
			ctx.Error(http.StatusMethodNotAllowed, "protected reference", err)
		} else {
			ctx.InternalServerError(err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}
