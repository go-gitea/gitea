// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package explore

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
)

const (
	tplExplorePackages templates.TplName = "explore/packages"
)

// Packages render explore packages page
func Packages(ctx *context.Context) {
	ctx.Data["UsersPageIsDisabled"] = setting.Service.Explore.DisableUsersPage
	ctx.Data["OrganizationsPageIsDisabled"] = setting.Service.Explore.DisableOrganizationsPage
	ctx.Data["CodePageIsDisabled"] = setting.Service.Explore.DisableCodePage
	ctx.Data["PackagesPageIsDisabled"] = setting.Service.Explore.DisablePackagesPage
	ctx.Data["PackagesEnabled"] = setting.Packages.Enabled
	ctx.Data["Title"] = ctx.Tr("explore")
	ctx.Data["PageIsExplore"] = true
	ctx.Data["PageIsExplorePackages"] = true

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	query := ctx.FormTrim("q")
	packageType := ctx.FormTrim("type")

	ctx.Data["Query"] = query
	ctx.Data["PackageType"] = packageType
	ctx.Data["AvailableTypes"] = packages_model.TypeList

	// Get all packages matching the search criteria
	pvs, total, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		Paginator: &db.ListOptions{
			PageSize: setting.UI.PackagesPagingNum * 3, // Get more to account for filtering
			Page:     page,
		},
		Type:       packages_model.Type(packageType),
		Name:       packages_model.SearchValue{Value: query},
		IsInternal: optional.Some(false),
	})
	if err != nil {
		ctx.ServerError("SearchLatestVersions", err)
		return
	}

	// Filter packages based on user permissions
	accessiblePVs := make([]*packages_model.PackageVersion, 0, len(pvs))
	for _, pv := range pvs {
		pkg, err := packages_model.GetPackageByID(ctx, pv.PackageID)
		if err != nil {
			ctx.ServerError("GetPackageByID", err)
			return
		}

		owner, err := user_model.GetUserByID(ctx, pkg.OwnerID)
		if err != nil {
			ctx.ServerError("GetUserByID", err)
			return
		}

		// Check if user has access to this package based on owner visibility
		hasAccess := false
		if owner.IsOrganization() {
			// For organizations, check if user can see the org
			if ctx.Doer != nil {
				isMember, err := org_model.IsOrganizationMember(ctx, owner.ID, ctx.Doer.ID)
				if err != nil {
					ctx.ServerError("IsOrganizationMember", err)
					return
				}
				hasAccess = isMember || owner.Visibility == structs.VisibleTypePublic
			} else {
				hasAccess = owner.Visibility == structs.VisibleTypePublic
			}
		} else {
			// For users, check visibility
			if ctx.Doer != nil {
				hasAccess = owner.Visibility == structs.VisibleTypePublic ||
					owner.Visibility == structs.VisibleTypeLimited ||
					owner.ID == ctx.Doer.ID
			} else {
				hasAccess = owner.Visibility == structs.VisibleTypePublic
			}
		}

		if hasAccess {
			accessiblePVs = append(accessiblePVs, pv)
			if len(accessiblePVs) >= setting.UI.PackagesPagingNum {
				break
			}
		}
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, accessiblePVs)
	if err != nil {
		ctx.ServerError("GetPackageDescriptors", err)
		return
	}

	ctx.Data["Total"] = int64(len(accessiblePVs))
	ctx.Data["PackageDescriptors"] = pds

	pager := context.NewPagination(int(total), setting.UI.PackagesPagingNum, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplExplorePackages)
}
