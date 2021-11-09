// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"sort"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"

	packages_service "code.gitea.io/gitea/services/packages"
)

const (
	tplPackages     base.TplName = "repo/packages/list"
	tplPackagesView base.TplName = "repo/packages/view"
)

// MustEnablePackages checks if packages are enabled
func MustEnablePackages(ctx *context.Context) {
	if unit.TypePackages.UnitGlobalDisabled() || !ctx.Repo.CanRead(unit.TypePackages) {
		ctx.NotFound("MustEnablePackages", nil)
	}
}

// Packages displays a list of all packages in the repository
func Packages(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.packages")
	ctx.Data["IsPackagesPage"] = true

	query := ctx.FormTrim("q")
	packageType := ctx.FormTrim("package_type")
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	repo := ctx.Repo.Repository

	pvs, count, err := packages.SearchLatestVersions(&packages.PackageSearchOptions{
		RepoID: repo.ID,
		Query:  query,
		Type:   packageType,
		Paginator: &db.ListOptions{
			Page:     page,
			PageSize: setting.UI.PackagesPagingNum,
		},
	})
	if err != nil {
		ctx.ServerError("SearchLatestVersions", err)
		return
	}

	pds, err := packages.GetPackageDescriptors(pvs)
	if err != nil {
		ctx.ServerError("GetPackageDescriptors", err)
		return
	}

	hasPackages, err := packages.HasRepositoryPackages(repo.ID)
	if err != nil {
		ctx.ServerError("HasRepositoryPackages", err)
		return
	}

	ctx.Data["HasPackages"] = hasPackages
	ctx.Data["PackageDescriptors"] = pds
	ctx.Data["Query"] = query
	ctx.Data["PackageType"] = packageType

	pager := context.NewPagination(int(count), setting.UI.PackagesPagingNum, page, 5)
	pager.AddParam(ctx, "q", "Query")
	pager.AddParam(ctx, "package_type", "PackageType")
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplPackages)
}

// ViewPackage displays a single package
func ViewPackage(ctx *context.Context) {
	pv, err := packages.GetVersionByID(db.DefaultContext, ctx.ParamsInt64(":id"))
	if err != nil {
		if err == packages.ErrPackageNotExist {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetVersionByID", err)
		}
		return
	}
	pd, err := packages.GetPackageDescriptor(pv)
	if err != nil {
		ctx.ServerError("GetPackageDescriptor", err)
		return
	}
	if pd.Repository.ID != ctx.Repo.Repository.ID {
		ctx.NotFound("", nil)
		return
	}

	ctx.Data["Title"] = pd.Package.Name
	ctx.Data["IsPackagesPage"] = true
	ctx.Data["PackageDescriptor"] = pd

	otherVersions, err := packages.GetVersionsByPackageName(ctx.Repo.Repository.ID, pd.Package.Type, pd.Package.LowerName)
	if err != nil {
		ctx.ServerError("GetVersionsByPackageName", err)
		return
	}
	sort.Slice(otherVersions, func(i, j int) bool {
		return otherVersions[i].Version > otherVersions[j].Version
	})
	ctx.Data["OtherVersions"] = otherVersions

	ctx.Data["Repo"] = ctx.Repo.Repository
	ctx.Data["CanWritePackages"] = ctx.Repo.Permission.CanWrite(unit.TypePackages)
	ctx.Data["PageIsPackages"] = true

	ctx.HTML(http.StatusOK, tplPackagesView)
}

// DeletePackagePost deletes a package
func DeletePackagePost(ctx *context.Context) {
	err := packages_service.DeleteVersionByID(ctx.User, ctx.Repo.Repository, ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.packages.delete.success"))
	}

	ctx.Redirect(ctx.Repo.RepoLink + "/packages")
}

// DownloadPackageFile serves the content of a package file
func DownloadPackageFile(ctx *context.Context) {
	s, pf, err := packages_service.GetFileStreamByPackageVersionID(ctx.Repo.Repository, ctx.ParamsInt64(":id"), ctx.Params(":filename"))
	if err != nil {
		if err == packages.ErrPackageNotExist || err == packages.ErrPackageFileNotExist {
			ctx.NotFound("", err)
			return
		}
		ctx.ServerError("GetFileStreamByPackageVersionID", err)
		return
	}
	defer s.Close()

	ctx.ServeStream(s, pf.Name)
}
