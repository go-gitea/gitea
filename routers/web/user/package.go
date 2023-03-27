// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	container_model "code.gitea.io/gitea/models/packages/container"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	debian_module "code.gitea.io/gitea/modules/packages/debian"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/forms"
	packages_service "code.gitea.io/gitea/services/packages"
)

const (
	tplPackagesList       base.TplName = "user/overview/packages"
	tplPackagesView       base.TplName = "package/view"
	tplPackageVersionList base.TplName = "user/overview/package_versions"
	tplPackagesSettings   base.TplName = "package/settings"
)

// ListPackages displays a list of all packages of the context user
func ListPackages(ctx *context.Context) {
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

	repositoryAccessMap := make(map[int64]bool)
	for _, pd := range pds {
		if pd.Repository == nil {
			continue
		}
		if _, has := repositoryAccessMap[pd.Repository.ID]; has {
			continue
		}

		permission, err := access_model.GetUserRepoPermission(ctx, pd.Repository, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetUserRepoPermission", err)
			return
		}
		repositoryAccessMap[pd.Repository.ID] = permission.HasAccess()
	}

	hasPackages, err := packages_model.HasOwnerPackages(ctx, ctx.ContextUser.ID)
	if err != nil {
		ctx.ServerError("HasOwnerPackages", err)
		return
	}

	shared_user.RenderUserHeader(ctx)

	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["IsPackagesPage"] = true
	ctx.Data["Query"] = query
	ctx.Data["PackageType"] = packageType
	ctx.Data["AvailableTypes"] = packages_model.TypeList
	ctx.Data["HasPackages"] = hasPackages
	ctx.Data["PackageDescriptors"] = pds
	ctx.Data["Total"] = total
	ctx.Data["RepositoryAccessMap"] = repositoryAccessMap

	// TODO: context/org -> HandleOrgAssignment() can not be used
	if ctx.ContextUser.IsOrganization() {
		org := org_model.OrgFromUser(ctx.ContextUser)
		ctx.Data["Org"] = org
		ctx.Data["OrgLink"] = ctx.ContextUser.OrganisationLink()

		if ctx.Doer != nil {
			ctx.Data["IsOrganizationMember"], _ = org_model.IsOrganizationMember(ctx, org.ID, ctx.Doer.ID)
			ctx.Data["IsOrganizationOwner"], _ = org_model.IsOrganizationOwner(ctx, org.ID, ctx.Doer.ID)
		} else {
			ctx.Data["IsOrganizationMember"] = false
			ctx.Data["IsOrganizationOwner"] = false
		}
	}

	pager := context.NewPagination(int(total), setting.UI.PackagesPagingNum, page, 5)
	pager.AddParam(ctx, "q", "Query")
	pager.AddParam(ctx, "type", "PackageType")
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplPackagesList)
}

// RedirectToLastVersion redirects to the latest package version
func RedirectToLastVersion(ctx *context.Context) {
	p, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packages_model.Type(ctx.Params("type")), ctx.Params("name"))
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			ctx.NotFound("GetPackageByName", err)
		} else {
			ctx.ServerError("GetPackageByName", err)
		}
		return
	}

	pvs, _, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		PackageID:  p.ID,
		IsInternal: util.OptionalBoolFalse,
	})
	if err != nil {
		ctx.ServerError("GetPackageByName", err)
		return
	}
	if len(pvs) == 0 {
		ctx.NotFound("", err)
		return
	}

	pd, err := packages_model.GetPackageDescriptor(ctx, pvs[0])
	if err != nil {
		ctx.ServerError("GetPackageDescriptor", err)
		return
	}

	ctx.Redirect(pd.FullWebLink())
}

// ViewPackageVersion displays a single package version
func ViewPackageVersion(ctx *context.Context) {
	pd := ctx.Package.Descriptor

	shared_user.RenderUserHeader(ctx)

	ctx.Data["Title"] = pd.Package.Name
	ctx.Data["IsPackagesPage"] = true
	ctx.Data["PackageDescriptor"] = pd

	switch pd.Package.Type {
	case packages_model.TypeContainer:
		ctx.Data["RegistryHost"] = setting.Packages.RegistryHost
	case packages_model.TypeDebian:
		distributions := make(container.Set[string])
		components := make(container.Set[string])
		architectures := make(container.Set[string])

		for _, f := range pd.Files {
			for _, pp := range f.Properties {
				switch pp.Name {
				case debian_module.PropertyDistribution:
					distributions.Add(pp.Value)
				case debian_module.PropertyComponent:
					components.Add(pp.Value)
				case debian_module.PropertyArchitecture:
					architectures.Add(pp.Value)
				}
			}
		}

		ctx.Data["Distributions"] = distributions.Values()
		ctx.Data["Components"] = components.Values()
		ctx.Data["Architectures"] = architectures.Values()
	}

	var (
		total int64
		pvs   []*packages_model.PackageVersion
		err   error
	)
	switch pd.Package.Type {
	case packages_model.TypeContainer:
		pvs, total, err = container_model.SearchImageTags(ctx, &container_model.ImageTagsSearchOptions{
			Paginator: db.NewAbsoluteListOptions(0, 5),
			PackageID: pd.Package.ID,
			IsTagged:  true,
		})
	default:
		pvs, total, err = packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
			Paginator:  db.NewAbsoluteListOptions(0, 5),
			PackageID:  pd.Package.ID,
			IsInternal: util.OptionalBoolFalse,
		})
	}
	if err != nil {
		ctx.ServerError("", err)
		return
	}

	ctx.Data["LatestVersions"] = pvs
	ctx.Data["TotalVersionCount"] = total

	ctx.Data["CanWritePackages"] = ctx.Package.AccessMode >= perm.AccessModeWrite || ctx.IsUserSiteAdmin()

	hasRepositoryAccess := false
	if pd.Repository != nil {
		permission, err := access_model.GetUserRepoPermission(ctx, pd.Repository, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetUserRepoPermission", err)
			return
		}
		hasRepositoryAccess = permission.HasAccess()
	}
	ctx.Data["HasRepositoryAccess"] = hasRepositoryAccess

	ctx.HTML(http.StatusOK, tplPackagesView)
}

// ListPackageVersions lists all versions of a package
func ListPackageVersions(ctx *context.Context) {
	p, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packages_model.Type(ctx.Params("type")), ctx.Params("name"))
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			ctx.NotFound("GetPackageByName", err)
		} else {
			ctx.ServerError("GetPackageByName", err)
		}
		return
	}

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}
	pagination := &db.ListOptions{
		PageSize: setting.UI.PackagesPagingNum,
		Page:     page,
	}

	query := ctx.FormTrim("q")
	sort := ctx.FormTrim("sort")

	shared_user.RenderUserHeader(ctx)

	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["IsPackagesPage"] = true
	ctx.Data["PackageDescriptor"] = &packages_model.PackageDescriptor{
		Package: p,
		Owner:   ctx.Package.Owner,
	}
	ctx.Data["Query"] = query
	ctx.Data["Sort"] = sort

	pagerParams := map[string]string{
		"q":    query,
		"sort": sort,
	}

	var (
		total int64
		pvs   []*packages_model.PackageVersion
	)
	switch p.Type {
	case packages_model.TypeContainer:
		tagged := ctx.FormTrim("tagged")

		pagerParams["tagged"] = tagged
		ctx.Data["Tagged"] = tagged

		pvs, total, err = container_model.SearchImageTags(ctx, &container_model.ImageTagsSearchOptions{
			Paginator: pagination,
			PackageID: p.ID,
			Query:     query,
			IsTagged:  tagged == "" || tagged == "tagged",
			Sort:      sort,
		})
		if err != nil {
			ctx.ServerError("SearchImageTags", err)
			return
		}
	default:
		pvs, total, err = packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
			Paginator: pagination,
			PackageID: p.ID,
			Version: packages_model.SearchValue{
				ExactMatch: false,
				Value:      query,
			},
			IsInternal: util.OptionalBoolFalse,
			Sort:       sort,
		})
		if err != nil {
			ctx.ServerError("SearchVersions", err)
			return
		}
	}

	ctx.Data["PackageDescriptors"], err = packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		ctx.ServerError("GetPackageDescriptors", err)
		return
	}

	ctx.Data["Total"] = total

	pager := context.NewPagination(int(total), setting.UI.PackagesPagingNum, page, 5)
	for k, v := range pagerParams {
		pager.AddParamString(k, v)
	}
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplPackageVersionList)
}

// PackageSettings displays the package settings page
func PackageSettings(ctx *context.Context) {
	pd := ctx.Package.Descriptor

	shared_user.RenderUserHeader(ctx)

	ctx.Data["Title"] = pd.Package.Name
	ctx.Data["IsPackagesPage"] = true
	ctx.Data["PackageDescriptor"] = pd

	repos, _, _ := repo_model.GetUserRepositories(&repo_model.SearchRepoOptions{
		Actor:   pd.Owner,
		Private: true,
	})
	ctx.Data["Repos"] = repos
	ctx.Data["CanWritePackages"] = ctx.Package.AccessMode >= perm.AccessModeWrite || ctx.IsUserSiteAdmin()

	ctx.HTML(http.StatusOK, tplPackagesSettings)
}

// PackageSettingsPost updates the package settings
func PackageSettingsPost(ctx *context.Context) {
	pd := ctx.Package.Descriptor

	form := web.GetForm(ctx).(*forms.PackageSettingForm)
	switch form.Action {
	case "link":
		success := func() bool {
			repoID := int64(0)
			if form.RepoID != 0 {
				repo, err := repo_model.GetRepositoryByID(ctx, form.RepoID)
				if err != nil {
					log.Error("Error getting repository: %v", err)
					return false
				}

				if repo.OwnerID != pd.Owner.ID {
					return false
				}

				repoID = repo.ID
			}

			if err := packages_model.SetRepositoryLink(ctx, pd.Package.ID, repoID); err != nil {
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
		err := packages_service.RemovePackageVersion(ctx.Doer, ctx.Package.Descriptor.Version)
		if err != nil {
			log.Error("Error deleting package: %v", err)
			ctx.Flash.Error(ctx.Tr("packages.settings.delete.error"))
		} else {
			ctx.Flash.Success(ctx.Tr("packages.settings.delete.success"))
		}

		ctx.Redirect(ctx.Package.Owner.HomeLink() + "/-/packages")
		return
	}
}

// DownloadPackageFile serves the content of a package file
func DownloadPackageFile(ctx *context.Context) {
	pf, err := packages_model.GetFileForVersionByID(ctx, ctx.Package.Descriptor.Version.ID, ctx.ParamsInt64(":fileid"))
	if err != nil {
		if err == packages_model.ErrPackageFileNotExist {
			ctx.NotFound("", err)
		} else {
			ctx.ServerError("GetFileForVersionByID", err)
		}
		return
	}

	s, _, err := packages_service.GetPackageFileStream(
		ctx,
		pf,
	)
	if err != nil {
		ctx.ServerError("GetPackageFileStream", err)
		return
	}
	defer s.Close()

	ctx.ServeContent(s, &context.ServeHeaderOptions{
		Filename:     pf.Name,
		LastModified: pf.CreatedUnix.AsLocalTime(),
	})
}
