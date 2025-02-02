// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"net/url"
	"time"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	packages_service "code.gitea.io/gitea/services/packages"
	packages_cleanup_service "code.gitea.io/gitea/services/packages/cleanup"
)

const (
	tplPackagesList templates.TplName = "admin/packages/list"
)

// Packages shows all packages
func Packages(ctx *context.Context) {
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}
	query := ctx.FormTrim("q")
	packageType := ctx.FormTrim("type")
	sort := ctx.FormTrim("sort")

	pvs, total, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		Type:       packages_model.Type(packageType),
		Name:       packages_model.SearchValue{Value: query},
		Sort:       sort,
		IsInternal: optional.Some(false),
		Paginator: &db.ListOptions{
			PageSize: setting.UI.PackagesPagingNum,
			Page:     page,
		},
	})
	if err != nil {
		ctx.ServerError("SearchVersions", err)
		return
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		ctx.ServerError("GetPackageDescriptors", err)
		return
	}

	totalBlobSize, err := packages_model.GetTotalBlobSize(ctx)
	if err != nil {
		ctx.ServerError("GetTotalBlobSize", err)
		return
	}

	totalUnreferencedBlobSize, err := packages_model.GetTotalUnreferencedBlobSize(ctx)
	if err != nil {
		ctx.ServerError("CalculateBlobSize", err)
		return
	}

	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["PageIsAdminPackages"] = true
	ctx.Data["Query"] = query
	ctx.Data["PackageType"] = packageType
	ctx.Data["AvailableTypes"] = packages_model.TypeList
	ctx.Data["SortType"] = sort
	ctx.Data["PackageDescriptors"] = pds
	ctx.Data["TotalCount"] = total
	ctx.Data["TotalBlobSize"] = totalBlobSize - totalUnreferencedBlobSize
	ctx.Data["TotalUnreferencedBlobSize"] = totalUnreferencedBlobSize

	pager := context.NewPagination(int(total), setting.UI.PackagesPagingNum, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplPackagesList)
}

// DeletePackageVersion deletes a package version
func DeletePackageVersion(ctx *context.Context) {
	pv, err := packages_model.GetVersionByID(ctx, ctx.FormInt64("id"))
	if err != nil {
		ctx.ServerError("GetRepositoryByID", err)
		return
	}

	if err := packages_service.RemovePackageVersion(ctx, ctx.Doer, pv); err != nil {
		ctx.ServerError("RemovePackageVersion", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("packages.settings.delete.success"))
	ctx.JSONRedirect(setting.AppSubURL + "/-/admin/packages?page=" + url.QueryEscape(ctx.FormString("page")) + "&q=" + url.QueryEscape(ctx.FormString("q")) + "&type=" + url.QueryEscape(ctx.FormString("type")))
}

func CleanupExpiredData(ctx *context.Context) {
	if err := packages_cleanup_service.CleanupExpiredData(ctx, time.Duration(0)); err != nil {
		ctx.ServerError("CleanupExpiredData", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("admin.packages.cleanup.success"))
	ctx.Redirect(setting.AppSubURL + "/-/admin/packages")
}
