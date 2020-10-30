// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	releaseservice "code.gitea.io/gitea/services/release"
)

// GetReleaseTag get a single release of a repository by its tagname
func GetReleaseTag(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/tags/{tag} repository repoGetReleaseTag
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
	//   description: tagname of the release to get
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Release"
	//   "404":
	//     "$ref": "#/responses/notFound"

	tag := ctx.Params(":tag")

	release, err := models.GetRelease(ctx.Repo.Repository.ID, tag)
	if err != nil {
		if models.IsErrReleaseNotExist(err) {
			ctx.Error(http.StatusNotFound, "GetRelease", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetRelease", err)
		return
	}

	if err := release.LoadAttributes(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToRelease(release))
}

// DeleteReleaseTag delete a tag from a repository
func DeleteReleaseTag(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/releases/tags/{tag} repository repoDeleteReleaseTag
	// ---
	// summary: Delete a release tag
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
	//   description: name of the tag to delete
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/conflict"

	tag := ctx.Params(":tag")

	release, err := models.GetRelease(ctx.Repo.Repository.ID, tag)
	if err != nil {
		if models.IsErrReleaseNotExist(err) {
			ctx.Error(http.StatusNotFound, "GetRelease", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetRelease", err)
		return
	}

	if !release.IsTag {
		ctx.Error(http.StatusConflict, "IsTag", errors.New("a tag attached to a release cannot be deleted directly"))
		return
	}

	if err := releaseservice.DeleteReleaseByID(release.ID, ctx.User, true); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteReleaseByID", err)
	}

	ctx.Status(http.StatusNoContent)
}
