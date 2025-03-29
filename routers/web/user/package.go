// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"
	"net/url"

	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	container_model "code.gitea.io/gitea/models/packages/container"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	alpine_module "code.gitea.io/gitea/modules/packages/alpine"
	arch_module "code.gitea.io/gitea/modules/packages/arch"
	debian_module "code.gitea.io/gitea/modules/packages/debian"
	rpm_module "code.gitea.io/gitea/modules/packages/rpm"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	packages_helper "code.gitea.io/gitea/routers/api/packages/helper"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	packages_service "code.gitea.io/gitea/services/packages"
)

const (
	tplPackagesList       templates.TplName = "user/overview/packages"
	tplPackagesView       templates.TplName = "package/view"
	tplPackageVersionList templates.TplName = "user/overview/package_versions"
	tplPackagesSettings   templates.TplName = "package/settings"
)

// ListPackages displays a list of all packages of the context user
func ListPackages(ctx *context.Context) {
	shared_user.PrepareContextForProfileBigAvatar(ctx)
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
		IsInternal: optional.Some(false),
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
		repositoryAccessMap[pd.Repository.ID] = permission.HasAnyUnitAccess()
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

	err = shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

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
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplPackagesList)
}

// RedirectToLastVersion redirects to the latest package version
func RedirectToLastVersion(ctx *context.Context) {
	p, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packages_model.Type(ctx.PathParam("type")), ctx.PathParam("name"))
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetPackageByName", err)
		}
		return
	}

	pvs, _, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		PackageID:  p.ID,
		IsInternal: optional.Some(false),
	})
	if err != nil {
		ctx.ServerError("GetPackageByName", err)
		return
	}
	if len(pvs) == 0 {
		ctx.NotFound(err)
		return
	}

	pd, err := packages_model.GetPackageDescriptor(ctx, pvs[0])
	if err != nil {
		ctx.ServerError("GetPackageDescriptor", err)
		return
	}

	ctx.Redirect(pd.VersionWebLink())
}

// ViewPackageVersion displays a single package version
func ViewPackageVersion(ctx *context.Context) {
	pd := ctx.Package.Descriptor

	shared_user.RenderUserHeader(ctx)

	ctx.Data["Title"] = pd.Package.Name
	ctx.Data["IsPackagesPage"] = true
	ctx.Data["PackageDescriptor"] = pd

	registryHostURL, err := url.Parse(httplib.GuessCurrentHostURL(ctx))
	if err != nil {
		registryHostURL, _ = url.Parse(setting.AppURL)
	}
	ctx.Data["PackageRegistryHost"] = registryHostURL.Host

	switch pd.Package.Type {
	case packages_model.TypeAlpine:
		branches := make(container.Set[string])
		repositories := make(container.Set[string])
		architectures := make(container.Set[string])

		for _, f := range pd.Files {
			for _, pp := range f.Properties {
				switch pp.Name {
				case alpine_module.PropertyBranch:
					branches.Add(pp.Value)
				case alpine_module.PropertyRepository:
					repositories.Add(pp.Value)
				case alpine_module.PropertyArchitecture:
					architectures.Add(pp.Value)
				}
			}
		}

		ctx.Data["Branches"] = util.Sorted(branches.Values())
		ctx.Data["Repositories"] = util.Sorted(repositories.Values())
		ctx.Data["Architectures"] = util.Sorted(architectures.Values())
	case packages_model.TypeArch:
		repositories := make(container.Set[string])
		architectures := make(container.Set[string])

		for _, f := range pd.Files {
			for _, pp := range f.Properties {
				switch pp.Name {
				case arch_module.PropertyRepository:
					repositories.Add(pp.Value)
				case arch_module.PropertyArchitecture:
					architectures.Add(pp.Value)
				}
			}
		}

		ctx.Data["Repositories"] = util.Sorted(repositories.Values())
		ctx.Data["Architectures"] = util.Sorted(architectures.Values())
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

		ctx.Data["Distributions"] = util.Sorted(distributions.Values())
		ctx.Data["Components"] = util.Sorted(components.Values())
		ctx.Data["Architectures"] = util.Sorted(architectures.Values())
	case packages_model.TypeRpm:
		groups := make(container.Set[string])
		architectures := make(container.Set[string])

		for _, f := range pd.Files {
			for _, pp := range f.Properties {
				switch pp.Name {
				case rpm_module.PropertyGroup:
					groups.Add(pp.Value)
				case rpm_module.PropertyArchitecture:
					architectures.Add(pp.Value)
				}
			}
		}

		ctx.Data["Groups"] = util.Sorted(groups.Values())
		ctx.Data["Architectures"] = util.Sorted(architectures.Values())
	}

	var (
		total int64
		pvs   []*packages_model.PackageVersion
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
			IsInternal: optional.Some(false),
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
		hasRepositoryAccess = permission.HasAnyUnitAccess()
	}
	ctx.Data["HasRepositoryAccess"] = hasRepositoryAccess

	err = shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	ctx.HTML(http.StatusOK, tplPackagesView)
}

// ListPackageVersions lists all versions of a package
func ListPackageVersions(ctx *context.Context) {
	shared_user.PrepareContextForProfileBigAvatar(ctx)
	p, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packages_model.Type(ctx.PathParam("type")), ctx.PathParam("name"))
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			ctx.NotFound(err)
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

	var (
		total int64
		pvs   []*packages_model.PackageVersion
	)
	switch p.Type {
	case packages_model.TypeContainer:
		tagged := ctx.FormTrim("tagged")

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
			IsInternal: optional.Some(false),
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

	err = shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	pager := context.NewPagination(int(total), setting.UI.PackagesPagingNum, page, 5)
	pager.AddParamFromRequest(ctx.Req)
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

	repos, _, _ := repo_model.GetUserRepositories(ctx, &repo_model.SearchRepoOptions{
		Actor:   pd.Owner,
		Private: true,
	})
	ctx.Data["Repos"] = repos
	ctx.Data["CanWritePackages"] = ctx.Package.AccessMode >= perm.AccessModeWrite || ctx.IsUserSiteAdmin()

	err := shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

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
		err := packages_service.RemovePackageVersion(ctx, ctx.Doer, ctx.Package.Descriptor.Version)
		if err != nil {
			log.Error("Error deleting package: %v", err)
			ctx.Flash.Error(ctx.Tr("packages.settings.delete.error"))
		} else {
			ctx.Flash.Success(ctx.Tr("packages.settings.delete.success"))
		}

		redirectURL := ctx.Package.Owner.HomeLink() + "/-/packages"
		// redirect to the package if there are still versions available
		if has, _ := packages_model.ExistVersion(ctx, &packages_model.PackageSearchOptions{PackageID: ctx.Package.Descriptor.Package.ID, IsInternal: optional.Some(false)}); has {
			redirectURL = ctx.Package.Descriptor.PackageWebLink()
		}

		ctx.Redirect(redirectURL)
		return
	}
}

// DownloadPackageFile serves the content of a package file
func DownloadPackageFile(ctx *context.Context) {
	pf, err := packages_model.GetFileForVersionByID(ctx, ctx.Package.Descriptor.Version.ID, ctx.PathParamInt64("fileid"))
	if err != nil {
		if err == packages_model.ErrPackageFileNotExist {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetFileForVersionByID", err)
		}
		return
	}

	s, u, _, err := packages_service.GetPackageFileStream(ctx, pf)
	if err != nil {
		ctx.ServerError("GetPackageFileStream", err)
		return
	}

	packages_helper.ServePackageFile(ctx, s, u, pf)
}
