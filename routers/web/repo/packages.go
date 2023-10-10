// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/web/shared/packages"
)

const (
	tplPackagesList base.TplName = "repo/packages"
)

// Packages displays a list of all packages in the repository
func Packages(ctx *context.Context) {
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}
	query := ctx.FormTrim("q")
	packageType := ctx.FormTrim("type")

	pvs, total, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		Paginator: &db.ListOptions{
			PageSize: setting.UI.PackagesPagingNum,
			Page:     page,
		},
		OwnerID:    ctx.ContextUser.ID,
		RepoID:     ctx.Repo.Repository.ID,
		Type:       packages_model.Type(packageType),
		Name:       packages_model.SearchValue{Value: query},
		IsInternal: util.OptionalBoolFalse,
	})
	if err != nil {
		ctx.ServerError("SearchLatestVersions", err)
		return
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		ctx.ServerError("GetPackageDescriptors", err)
		return
	}

	hasPackages, err := packages_model.HasRepositoryPackages(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("HasRepositoryPackages", err)
		return
	}

	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["IsPackagesPage"] = true
	ctx.Data["Query"] = query
	ctx.Data["PackageType"] = packageType
	ctx.Data["AvailableTypes"] = packages_model.TypeList
	ctx.Data["HasPackages"] = hasPackages
	if ctx.Repo != nil {
		ctx.Data["CanWritePackages"] = ctx.IsUserRepoWriter([]unit.Type{unit.TypePackages}) || ctx.IsUserSiteAdmin()
		ctx.Data["ShowPackageUploadButton"] = ctx.Data["CanWritePackages"]
		ctx.Data["PackageUploadUrl"] = fmt.Sprintf("%s/-/packages/upload/", ctx.Repo.Owner.HTMLURL())
		ctx.Data["PackageUploadRepo"] = ctx.Repo.Repository.Name
		ctx.Data["AllowedUploadTypes"] = packages.UploadTypeList
	}
	ctx.Data["PackageDescriptors"] = pds
	ctx.Data["Total"] = total
	ctx.Data["RepositoryAccessMap"] = map[int64]bool{ctx.Repo.Repository.ID: true} // There is only the current repository

	pager := context.NewPagination(int(total), setting.UI.PackagesPagingNum, page, 5)
	pager.AddParam(ctx, "q", "Query")
	pager.AddParam(ctx, "type", "PackageType")
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplPackagesList)
}
