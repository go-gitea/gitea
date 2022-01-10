// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"net/http"
	"sort"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	packages_service "code.gitea.io/gitea/services/packages"
)

const (
	tplPackagesList     base.TplName = "user/overview/packages"
	tplPackagesView     base.TplName = "package/view"
	tplPackagesSettings base.TplName = "package/settings"
)

// Packages displays a list of all packages of the context user
func Packages(ctx *context.Context) {
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}
	query := ctx.FormTrim("q")
	packageType := ctx.FormTrim("type")

	pvs, total, err := packages.SearchLatestVersions(&packages.PackageSearchOptions{
		Paginator: &db.ListOptions{
			PageSize: setting.UI.PackagesPagingNum,
			Page:     page,
		},
		OwnerID: ctx.ContextUser.ID,
		Query:   query,
		Type:    packageType,
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

	hasPackages, err := packages.HasOwnerPackages(db.DefaultContext, ctx.ContextUser.ID)
	if err != nil {
		ctx.ServerError("HasOwnerPackages", err)
		return
	}

	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["IsPackagesPage"] = true
	ctx.Data["ContextUser"] = ctx.ContextUser
	ctx.Data["Query"] = query
	ctx.Data["PackageType"] = packageType
	ctx.Data["HasPackages"] = hasPackages
	ctx.Data["PackageDescriptors"] = pds
	ctx.Data["Total"] = total

	pager := context.NewPagination(int(total), setting.UI.PackagesPagingNum, page, 5)
	pager.AddParam(ctx, "q", "Query")
	pager.AddParam(ctx, "type", "PackageType")
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplPackagesList)
}

// ViewPackage displays a single package
func ViewPackage(ctx *context.Context) {
	pd := ctx.Package.Descriptor

	if pd == nil {
		ctx.NotFound("Package does not exist", nil)
		return
	}

	ctx.Data["Title"] = pd.Package.Name
	ctx.Data["IsPackagesPage"] = true
	ctx.Data["ContextUser"] = ctx.ContextUser
	ctx.Data["PackageDescriptor"] = pd

	otherVersions, err := packages.GetVersionsByPackageName(pd.Owner.ID, pd.Package.Type, pd.Package.LowerName)
	if err != nil {
		ctx.ServerError("GetVersionsByPackageName", err)
		return
	}
	sort.Slice(otherVersions, func(i, j int) bool {
		return otherVersions[i].CreatedUnix > otherVersions[j].CreatedUnix
	})
	ctx.Data["OtherVersions"] = otherVersions

	ctx.Data["CanWritePackages"] = ctx.Package.AccessMode >= perm.AccessModeWrite || ctx.IsUserSiteAdmin()

	ctx.HTML(http.StatusOK, tplPackagesView)
}

// PackageSettings displays the package settings page
func PackageSettings(ctx *context.Context) {
	pd := ctx.Package.Descriptor

	if pd == nil {
		ctx.NotFound("Package does not exist", nil)
		return
	}

	ctx.Data["Title"] = pd.Package.Name
	ctx.Data["IsPackagesPage"] = true
	ctx.Data["ContextUser"] = ctx.ContextUser
	ctx.Data["PackageDescriptor"] = pd

	repos, _, _ := models.GetUserRepositories(&models.SearchRepoOptions{
		Actor: pd.Owner,
	})
	ctx.Data["Repos"] = repos
	ctx.Data["CanWritePackages"] = ctx.Package.AccessMode >= perm.AccessModeWrite || ctx.IsUserSiteAdmin()

	ctx.HTML(http.StatusOK, tplPackagesSettings)
}

// PackageSettingsPost updates the package settings
func PackageSettingsPost(ctx *context.Context) {
	pd := ctx.Package.Descriptor

	if pd == nil {
		ctx.NotFound("Package does not exist", nil)
		return
	}

	form := web.GetForm(ctx).(*forms.PackageSettingForm)
	switch form.Action {
	case "link":
		success := func() bool {
			repo, err := repo_model.GetRepositoryByID(form.RepoID)
			if err != nil {
				log.Error("Error getting repository: %v", err)
				return false
			}

			if repo.OwnerID != pd.Owner.ID {
				return false
			}

			if err = packages.SetRepositoryLink(pd.Package.ID, repo.ID); err != nil {
				log.Error("Error updating package: %v", err)
				return false
			}

			return true
		}()

		if success {
			ctx.Flash.Success(ctx.Tr("packages.settings.link.success"))
		} else {
			ctx.Flash.Error(ctx.Tr("packages.settings.link.error"))
		}

		ctx.Redirect(ctx.Link)
		return
	case "delete":
		err := packages_service.DeletePackageVersion(ctx.User, ctx.Package.Descriptor.Version)
		if err != nil {
			log.Error("Error deleting package: %v", err)
			ctx.Flash.Error(ctx.Tr("packages.settings.delete.error"))
		} else {
			ctx.Flash.Success(ctx.Tr("packages.settings.delete.success"))
		}

		ctx.Redirect(ctx.Package.Owner.HTMLURL() + "/-/packages")
		return
	}
}

// DownloadPackageFile serves the content of a package file
func DownloadPackageFile(ctx *context.Context) {
	s, pf, err := packages_service.GetFileStreamByPackageVersionID(ctx.ContextUser, ctx.ParamsInt64(":versionid"), ctx.Params(":filename"))
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
