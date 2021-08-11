// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"sort"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/packages/maven"
	"code.gitea.io/gitea/modules/packages/npm"
	"code.gitea.io/gitea/modules/packages/nuget"
	"code.gitea.io/gitea/modules/packages/pypi"
	"code.gitea.io/gitea/modules/setting"

	package_service "code.gitea.io/gitea/services/packages"
)

const (
	tplPackages     base.TplName = "repo/packages/list"
	tplPackagesView base.TplName = "repo/packages/view"
)

// MustEnablePackages checks if packages are enabled
func MustEnablePackages(ctx *context.Context) {
	if models.UnitTypePackages.UnitGlobalDisabled() || !ctx.Repo.CanRead(models.UnitTypePackages) {
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

	packages, count, err := models.GetLatestPackagesGrouped(&models.PackageSearchOptions{
		RepoID: repo.ID,
		Query:  query,
		Type:   packageType,
		Paginator: &models.ListOptions{
			Page:     page,
			PageSize: setting.UI.PackagesPagingNum,
		},
	})
	if err != nil {
		ctx.ServerError("GetLatestPackagesGrouped", err)
		return
	}

	for _, p := range packages {
		if err := p.LoadCreator(); err != nil {
			ctx.ServerError("LoadCreator", err)
			return
		}
	}

	ctx.Data["Packages"] = packages
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
	p, err := models.GetPackageByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if err == models.ErrPackageNotExist {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetPackageByID", err)
		}
		return
	}
	if p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("", nil)
		return
	}
	if err := p.LoadCreator(); err != nil {
		ctx.ServerError("LoadCreator", err)
		return
	}

	ctx.Data["Title"] = p.Name
	ctx.Data["IsPackagesPage"] = true
	ctx.Data["Package"] = p

	var metadata interface{}
	switch p.Type {
	case models.PackageNuGet:
		metadata = &nuget.Metadata{}
	case models.PackageNpm:
		metadata = &npm.Metadata{}
	case models.PackageMaven:
		metadata = &maven.Metadata{}
	case models.PackagePyPI:
		metadata = &pypi.Metadata{}
	}
	if metadata != nil {
		if err := json.Unmarshal([]byte(p.MetadataRaw), &metadata); err != nil {
			ctx.ServerError("Unmarshal", err)
			return
		}
	}
	ctx.Data["Metadata"] = metadata

	files, err := p.GetFiles()
	if err != nil {
		ctx.ServerError("GetFiles", err)
		return
	}
	ctx.Data["Files"] = files

	otherVersions, err := models.GetPackagesByName(ctx.Repo.Repository.ID, p.Type, p.LowerName)
	if err != nil {
		ctx.ServerError("GetPackagesByName", err)
		return
	}
	sort.Slice(otherVersions, func(i, j int) bool {
		return otherVersions[i].Version > otherVersions[j].Version
	})
	ctx.Data["OtherVersions"] = otherVersions

	ctx.Data["Repo"] = ctx.Repo.Repository
	ctx.Data["CanWritePackages"] = ctx.Repo.Permission.CanWrite(models.UnitTypePackages)
	ctx.Data["PageIsPackages"] = true

	ctx.HTML(http.StatusOK, tplPackagesView)
}

// DeletePackagePost deletes a package
func DeletePackagePost(ctx *context.Context) {
	err := package_service.DeletePackageByID(ctx.User, ctx.Repo.Repository, ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.packages.delete.success"))
	}

	ctx.Redirect(ctx.Repo.RepoLink + "/packages")
}

// DownloadPackageFile serves the content of a package file
func DownloadPackageFile(ctx *context.Context) {
	s, pf, err := package_service.GetFileStreamByPackageID(ctx.Repo.Repository, ctx.ParamsInt64(":id"), ctx.Params(":filename"))
	if err != nil {
		if err == models.ErrPackageNotExist || err == models.ErrPackageFileNotExist {
			ctx.NotFound("", err)
			return
		}
		ctx.ServerError("GetFileStreamByPackageID", err)
		return
	}
	defer s.Close()

	ctx.ServeStream(s, pf.Name)
}
