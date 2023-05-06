// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"net/http"

	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
	packages_service "code.gitea.io/gitea/services/packages"
)

// ListPackages gets all packages of an owner
func ListPackages(ctx *context.APIContext) {
	// swagger:operation GET /packages/{owner} package listPackages
	// ---
	// summary: Gets all packages of an owner
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the packages
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
	// - name: type
	//   in: query
	//   description: package type filter
	//   type: string
	//   enum: [alpine, cargo, chef, composer, conan, conda, container, debian, generic, helm, maven, npm, nuget, pub, pypi, rpm, rubygems, swift, vagrant]
	// - name: q
	//   in: query
	//   description: name filter
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/PackageList"

	listOptions := utils.GetListOptions(ctx)

	packageType := ctx.FormTrim("type")
	query := ctx.FormTrim("q")

	pvs, count, err := packages.SearchVersions(ctx, &packages.PackageSearchOptions{
		OwnerID:    ctx.Package.Owner.ID,
		Type:       packages.Type(packageType),
		Name:       packages.SearchValue{Value: query},
		IsInternal: util.OptionalBoolFalse,
		Paginator:  &listOptions,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SearchVersions", err)
		return
	}

	pds, err := packages.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetPackageDescriptors", err)
		return
	}

	apiPackages := make([]*api.Package, 0, len(pds))
	for _, pd := range pds {
		apiPackage, err := convert.ToPackage(ctx, pd, ctx.Doer)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "Error converting package for api", err)
			return
		}
		apiPackages = append(apiPackages, apiPackage)
	}

	ctx.SetLinkHeader(int(count), listOptions.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiPackages)
}

// GetPackage gets a package
func GetPackage(ctx *context.APIContext) {
	// swagger:operation GET /packages/{owner}/{type}/{name}/{version} package getPackage
	// ---
	// summary: Gets a package
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the package
	//   type: string
	//   required: true
	// - name: type
	//   in: path
	//   description: type of the package
	//   type: string
	//   required: true
	// - name: name
	//   in: path
	//   description: name of the package
	//   type: string
	//   required: true
	// - name: version
	//   in: path
	//   description: version of the package
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Package"
	//   "404":
	//     "$ref": "#/responses/notFound"

	apiPackage, err := convert.ToPackage(ctx, ctx.Package.Descriptor, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Error converting package for api", err)
		return
	}

	ctx.JSON(http.StatusOK, apiPackage)
}

// DeletePackage deletes a package
func DeletePackage(ctx *context.APIContext) {
	// swagger:operation DELETE /packages/{owner}/{type}/{name}/{version} package deletePackage
	// ---
	// summary: Delete a package
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the package
	//   type: string
	//   required: true
	// - name: type
	//   in: path
	//   description: type of the package
	//   type: string
	//   required: true
	// - name: name
	//   in: path
	//   description: name of the package
	//   type: string
	//   required: true
	// - name: version
	//   in: path
	//   description: version of the package
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	err := packages_service.RemovePackageVersion(ctx.Doer, ctx.Package.Descriptor.Version)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "RemovePackageVersion", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ListPackageFiles gets all files of a package
func ListPackageFiles(ctx *context.APIContext) {
	// swagger:operation GET /packages/{owner}/{type}/{name}/{version}/files package listPackageFiles
	// ---
	// summary: Gets all files of a package
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the package
	//   type: string
	//   required: true
	// - name: type
	//   in: path
	//   description: type of the package
	//   type: string
	//   required: true
	// - name: name
	//   in: path
	//   description: name of the package
	//   type: string
	//   required: true
	// - name: version
	//   in: path
	//   description: version of the package
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PackageFileList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	apiPackageFiles := make([]*api.PackageFile, 0, len(ctx.Package.Descriptor.Files))
	for _, pfd := range ctx.Package.Descriptor.Files {
		apiPackageFiles = append(apiPackageFiles, convert.ToPackageFile(pfd))
	}

	ctx.JSON(http.StatusOK, apiPackageFiles)
}
