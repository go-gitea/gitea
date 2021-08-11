// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	package_service "code.gitea.io/gitea/services/packages"
)

// ListPackages gets all packages of a repository
func ListPackages(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/packages repository repoListPackages
	// ---
	// summary: Gets all packages of a repository
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
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// - name: package_type
	//   in: query
	//   description: page size of results
	//   schema:
	//     type: string
	//     enum: [generic, nuget, npm, maven, pypi]
	// - name: q
	//   in: query
	//   description: name filter
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/PackageList"

	listOptions := utils.GetListOptions(ctx)

	packageType := ctx.FormTrim("package_type")
	query := ctx.FormTrim("q")

	repo := ctx.Repo.Repository

	packages, count, err := models.GetPackages(models.PackageSearchOptions{
		RepoID:      repo.ID,
		Type:        packageType,
		Query:       query,
		ListOptions: listOptions,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetPackages", err)
		return
	}

	apiPackages := make([]*api.Package, 0, len(packages))
	for _, p := range packages {
		if err := p.LoadCreator(); err != nil {
			ctx.Error(http.StatusInternalServerError, "LoadCreator", err)
			return
		}
		apiPackages = append(apiPackages, convert.ToPackage(p))
	}

	ctx.SetLinkHeader(int(count), listOptions.PageSize)
	ctx.Header().Set("X-Total-Count", fmt.Sprint(count))
	ctx.Header().Set("Access-Control-Expose-Headers", "X-Total-Count, Link")
	ctx.JSON(http.StatusOK, apiPackages)
}

// GetPackage gets a package
func GetPackage(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/packages/{id} repository repoGetPackage
	// ---
	// summary: Gets a package
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
	//   description: id of the package
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Package"
	//   "404":
	//     "$ref": "#/responses/notFound"

	p, err := models.GetPackageByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if err == models.ErrPackageNotExist {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPackageByID", err)
		}
		return
	}

	ctx.JSON(http.StatusOK, convert.ToPackage(p))
}

// DeletePackage delete a package from a repository
func DeletePackage(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/packages/{id} repository repoDeletePackage
	// ---
	// summary: Delete a package from a repository
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
	//   description: id of the package to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	err := package_service.DeletePackageByID(ctx.User, ctx.Repo.Repository, ctx.ParamsInt64(":id"))
	if err != nil {
		if err == models.ErrPackageNotExist {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "DeletePackageByID", err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ListPackageFiles gets all files of a package
func ListPackageFiles(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/packages/{id}/files repository repoListPackageFiles
	// ---
	// summary: Gets all files of a package
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
	//   description: id of the package to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PackageFileList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	p, err := models.GetPackageByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if err == models.ErrPackageNotExist {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPackageByID", err)
		}
		return
	}

	files, err := p.GetFiles()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetFiles", err)
		return
	}

	apiPackageFiles := make([]*api.PackageFile, 0, len(files))
	for _, pf := range files {
		apiPackageFiles = append(apiPackageFiles, convert.ToPackageFile(pf))
	}

	ctx.JSON(http.StatusOK, apiPackageFiles)
}
