// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	releaseservice "code.gitea.io/gitea/services/release"
)

// GetReleaseByTag get a single release of a repository by tag name
func GetReleaseByTag(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/tags/{tag} repository repoGetReleaseByTag
	// ---
	// summary: Get a release by tag name
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
	// - name: tag
	//   in: path
	//   description: tag name of the release to get
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Release"
	//   "404":
	//     "$ref": "#/responses/notFound"

	tag := ctx.Params(":tag")

	release, err := repo_model.GetRelease(ctx, ctx.Repo.Repository.ID, tag)
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			ctx.NotFound()
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetRelease", err)
		return
	}

	if release.IsTag {
		ctx.NotFound()
		return
	}

	if err = release.LoadAttributes(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToAPIRelease(ctx, ctx.Repo.Repository, release))
}

// DeleteReleaseByTag delete a release from a repository by tag name
func DeleteReleaseByTag(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/releases/tags/{tag} repository repoDeleteReleaseByTag
	// ---
	// summary: Delete a release by tag name
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
	// - name: tag
	//   in: path
	//   description: tag name of the release to delete
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	tag := ctx.Params(":tag")

	release, err := repo_model.GetRelease(ctx, ctx.Repo.Repository.ID, tag)
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			ctx.NotFound()
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetRelease", err)
		return
	}

	if release.IsTag {
		ctx.NotFound()
		return
	}

	if err = releaseservice.DeleteReleaseByID(ctx, ctx.Repo.Repository, release, ctx.Doer, false); err != nil {
		if models.IsErrProtectedTagName(err) {
			ctx.Error(http.StatusUnprocessableEntity, "delTag", "user not allowed to delete protected tag")
			return
		}
		ctx.Error(http.StatusInternalServerError, "DeleteReleaseByID", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
