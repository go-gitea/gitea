// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
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
	//   description: package type filter
	//   type: string
	//   enum: [generic, nuget, npm, maven, pypi]
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

	pvs, count, err := packages.SearchVersions(&packages.PackageSearchOptions{
		RepoID:    repo.ID,
		Type:      packageType,
		Query:     query,
		Paginator: &listOptions,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SearchVersions", err)
		return
	}

	pds, err := packages.GetPackageDescriptors(pvs)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetPackageDescriptors", err)
		return
	}

	apiPackages := make([]*api.Package, 0, len(pds))
	for _, pd := range pds {
		apiPackages = append(apiPackages, convert.ToPackage(pd))
	}

	ctx.SetLinkHeader(int(count), listOptions.PageSize)
	ctx.SetTotalCountHeader(count)
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

	pv, err := packages.GetVersionByID(db.DefaultContext, ctx.ParamsInt64(":id"))
	if err != nil {
		if err == packages.ErrPackageNotExist {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetVersionByID", err)
		}
		return
	}

	pd, err := packages.GetPackageDescriptor(pv)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetPackageDescriptor", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToPackage(pd))
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
	//   description: id of the package
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	err := package_service.DeleteVersionByID(ctx.User, ctx.Repo.Repository, ctx.ParamsInt64(":id"))
	if err != nil {
		if err == packages.ErrPackageNotExist {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteVersionByID", err)
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
	//   description: id of the package
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PackageFileList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	pv, err := packages.GetVersionByID(db.DefaultContext, ctx.ParamsInt64(":id"))
	if err != nil {
		if err == packages.ErrPackageNotExist {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetVersionByID", err)
		}
		return
	}

	pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pv.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetFilesByVersionID", err)
		return
	}

	apiPackageFiles := make([]*api.PackageFile, 0, len(pfs))
	for _, pf := range pfs {
		pb, err := packages.GetBlobByID(db.DefaultContext, pf.BlobID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetBlobByID", err)
			return
		}
		apiPackageFiles = append(apiPackageFiles, convert.ToPackageFile(pf, pb))
	}

	ctx.JSON(http.StatusOK, apiPackageFiles)
}
