// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
)

// GetPackage get a single package of a repository
func GetPackage(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/packages/{type}/{name} repository repoGetPackage
	// ---
	// summary: Get a package
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
	// - name: type
	//   in: path
	//   description: type of package
	//   type: string
	//   required: true
	// - name: name
	//   in: path
	//   description: name of package
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Package"
	//   "404":
	//     "$ref": "#/responses/notFound"
	typ := ctx.Params("type")
	name := ctx.Params("name")
	if typ != models.PackageTypeDockerImage.String() {
		ctx.NotFound()
		return
	}
	pkg, err := models.GetPackage(ctx.Repo.Repository.ID, models.PackageTypeDockerImage, name)
	if err != nil {
		if models.IsErrPackageNotExist(err) {
			ctx.NotFound()
		}
		ctx.InternalServerError(err)
		return
	}

	if err := pkg.LoadRepo(); err != nil {
		ctx.InternalServerError(err)
		return
	}

	if err := pkg.Repo.GetOwner(); err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToPackage(pkg))
}
