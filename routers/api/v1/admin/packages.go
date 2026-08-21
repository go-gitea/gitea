// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	packages_model "gitea.dev/models/packages"
	"gitea.dev/modules/optional"
	api "gitea.dev/modules/structs"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

// ListPackages lists every package version on the instance.
func ListPackages(ctx *context.APIContext) {
	// swagger:operation GET /admin/packages admin adminListPackages
	// ---
	// summary: List all packages
	// produces:
	// - application/json
	// parameters:
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
	//   enum: [alpine, cargo, chef, composer, conan, conda, container, cran, debian, generic, go, helm, maven, npm, nuget, pub, pypi, rpm, rubygems, swift, terraform, vagrant]
	// - name: q
	//   in: query
	//   description: name filter
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/PackageList"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	listOptions := utils.GetListOptions(ctx)

	apiPackages, count, err := searchPackages(ctx, &packages_model.PackageSearchOptions{
		Type:       packages_model.Type(ctx.FormTrim("type")),
		Name:       packages_model.SearchValue{Value: ctx.FormTrim("q")},
		IsInternal: optional.Some(false),
		Paginator:  &listOptions,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetLinkHeader(count, listOptions.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiPackages)
}

func searchPackages(ctx *context.APIContext, opts *packages_model.PackageSearchOptions) ([]*api.Package, int64, error) {
	pvs, count, err := packages_model.SearchVersions(ctx, opts)
	if err != nil {
		return nil, 0, err
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		return nil, 0, err
	}

	apiPackages := make([]*api.Package, 0, len(pds))
	for _, pd := range pds {
		apiPackage, err := convert.ToPackage(ctx, pd, ctx.Doer)
		if err != nil {
			return nil, 0, err
		}
		apiPackages = append(apiPackages, apiPackage)
	}

	return apiPackages, count, nil
}
