// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"gitea.dev/models/organization"
	packages_model "gitea.dev/models/packages"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
)

// Package contains owner, optional team, access mode and optional the package descriptor
type Package struct {
	Owner      *user_model.User
	Team       *organization.Team
	AccessMode perm.AccessMode
	Descriptor *packages_model.PackageDescriptor
}

func (p *Package) TeamID() int64 {
	if p.Team != nil {
		return p.Team.ID
	}
	return 0
}

func (p *Package) RegistryRootPath() string {
	if p.Team != nil {
		return fmt.Sprintf("%s/teams/%s", url.PathEscape(p.Owner.Name), url.PathEscape(p.Team.LowerName))
	}
	return url.PathEscape(p.Owner.Name)
}

func (p *Package) ContainerRootPath() string {
	if p.Team != nil {
		return fmt.Sprintf("%s/teams/%s", p.Owner.LowerName, p.Team.LowerName)
	}
	return p.Owner.LowerName
}

func (p *Package) PackagesListWebLink() string {
	if p.Team != nil {
		return fmt.Sprintf("/org/%s/teams/%s/packages", url.PathEscape(p.Owner.LowerName), url.PathEscape(p.Team.LowerName))
	}
	return p.Owner.HomeLink() + "/-/packages"
}

type packageAssignmentCtx struct {
	*Base
	Doer        *user_model.User
	ContextUser *user_model.User
}

// PackageAssignment returns a middleware to handle Context.Package assignment
func PackageAssignment() func(ctx *Context) {
	return func(ctx *Context) {
		errorFn := func(status int, msg string) {
			err := fmt.Errorf("%s", msg)
			if status == http.StatusNotFound {
				ctx.NotFound(err)
			} else {
				ctx.ServerError("PackageAssignment", err)
			}
		}
		paCtx := &packageAssignmentCtx{Base: ctx.Base, Doer: ctx.Doer, ContextUser: ctx.ContextUser}
		ctx.Package = packageAssignment(paCtx, errorFn)
	}
}

// PackageAssignmentAPI returns a middleware to handle Context.Package assignment
func PackageAssignmentAPI() func(ctx *APIContext) {
	return func(ctx *APIContext) {
		paCtx := &packageAssignmentCtx{Base: ctx.Base, Doer: ctx.Doer, ContextUser: ctx.ContextUser}
		ctx.Package = packageAssignment(paCtx, ctx.APIError)
	}
}

func packageAssignment(ctx *packageAssignmentCtx, errCb func(int, string)) *Package {
	pkgOwner := ctx.ContextUser
	teamName := ctx.PathParam("team")
	var pkgTeam *organization.Team
	if teamName != "" {
		if !pkgOwner.IsOrganization() {
			errCb(http.StatusNotFound, "Owner is not an organization")
			return nil
		}
		var err error
		pkgTeam, err = organization.GetTeam(ctx, pkgOwner.ID, teamName)
		if err != nil {
			if errors.Is(err, util.ErrNotExist) || organization.IsErrTeamNotExist(err) {
				errCb(http.StatusNotFound, fmt.Sprintf("GetTeam: %v", err))
			} else {
				errCb(http.StatusInternalServerError, fmt.Sprintf("GetTeam: %v", err))
			}
			return nil
		}
	}

	accessMode, err := determineAccessMode(ctx.Base, pkgOwner, pkgTeam, ctx.Doer)
	if err != nil {
		errCb(http.StatusInternalServerError, fmt.Sprintf("determineAccessMode: %v", err))
		return nil
	}

	pkg := &Package{
		Owner:      pkgOwner,
		Team:       pkgTeam,
		AccessMode: accessMode,
	}
	packageType := ctx.PathParam("type")
	name := ctx.PathParam("name")
	if packageType == "" || name == "" {
		return pkg
	}

	version := ctx.PathParam("version")
	if version != "" {
		pv, err := packages_model.GetTeamVersionByNameAndVersion(ctx, pkg.Owner.ID, pkg.TeamID(), packages_model.Type(packageType), name, version)
		if err != nil {
			if errors.Is(err, packages_model.ErrPackageNotExist) {
				errCb(http.StatusNotFound, fmt.Sprintf("GetTeamVersionByNameAndVersion: %v", err))
			} else {
				errCb(http.StatusInternalServerError, fmt.Sprintf("GetTeamVersionByNameAndVersion: %v", err))
			}
			return pkg
		}

		pkg.Descriptor, err = packages_model.GetPackageDescriptor(ctx, pv)
		if err != nil {
			errCb(http.StatusInternalServerError, fmt.Sprintf("GetPackageDescriptor: %v", err))
			return pkg
		}
	} else {
		p, err := packages_model.GetTeamPackageByName(ctx, pkg.Owner.ID, pkg.TeamID(), packages_model.Type(packageType), name)
		if err != nil {
			if errors.Is(err, packages_model.ErrPackageNotExist) {
				errCb(http.StatusNotFound, fmt.Sprintf("GetTeamPackageByName: %v", err))
			} else {
				errCb(http.StatusInternalServerError, fmt.Sprintf("GetTeamPackageByName: %v", err))
			}
			return pkg
		}

		pkg.Descriptor = &packages_model.PackageDescriptor{
			Package: p,
			Owner:   pkg.Owner,
			Team:    pkg.Team,
		}
	}

	return pkg
}

func determineAccessMode(ctx *Base, pkgOwner *user_model.User, pkgTeam *organization.Team, doer *user_model.User) (perm.AccessMode, error) {
	if !user_model.DoerTokenAllowsOwner(doer, pkgOwner) {
		return perm.AccessModeNone, nil
	}
	if pkgTeam != nil && user_model.IsPublicOnlyDoer(doer) && !pkgTeam.IsPublic() {
		return perm.AccessModeNone, nil
	}
	if user_model.DoerIsSiteAdminFullAccess(doer) {
		return perm.AccessModeOwner, nil
	}

	if setting.Service.RequireSignInViewStrict && (doer == nil || doer.IsGhost()) {
		return perm.AccessModeNone, nil
	}

	if doer != nil && !doer.IsGhost() && (!doer.IsActive || doer.ProhibitLogin) {
		return perm.AccessModeNone, nil
	}

	accessMode := perm.AccessModeNone
	if pkgTeam != nil {
		if doer != nil && !doer.IsGhost() {
			isOrgOwner, err := organization.IsOrganizationOwner(ctx, pkgOwner.ID, doer.ID)
			if err != nil {
				return accessMode, err
			}
			if isOrgOwner {
				return perm.AccessModeOwner, nil
			}

			isTeamMember, err := organization.IsTeamMember(ctx, pkgOwner.ID, pkgTeam.ID, doer.ID)
			if err != nil {
				return accessMode, err
			}
			if isTeamMember {
				accessMode = pkgTeam.UnitAccessMode(ctx, unit.TypePackages)
			}
		}

		if accessMode == perm.AccessModeNone {
			canRead, err := pkgTeam.CanNonMemberReadMeta(ctx, pkgOwner, doer)
			if err != nil {
				return accessMode, err
			}
			if canRead {
				accessMode = perm.AccessModeRead
			}
		}
		return accessMode, nil
	}

	if pkgOwner.IsOrganization() {
		org := organization.OrgFromUser(pkgOwner)

		if doer != nil && !doer.IsGhost() {
			// 1. If user is logged in, check all team packages permissions
			var err error
			accessMode, err = org.GetOrgUserMaxAuthorizeLevel(ctx, doer.ID)
			if err != nil {
				return accessMode, err
			}
			// If access mode is less than write check every team for more permissions
			// The minimum possible access mode is read for org members
			if accessMode < perm.AccessModeWrite {
				teams, err := organization.GetUserOrgTeams(ctx, org.ID, doer.ID)
				if err != nil {
					return accessMode, err
				}
				for _, t := range teams {
					perm := t.UnitAccessMode(ctx, unit.TypePackages)
					if accessMode < perm {
						accessMode = perm
					}
				}
			}
		}
		if accessMode == perm.AccessModeNone && organization.HasOrgOrUserVisible(ctx, pkgOwner, doer) {
			// 2. If user is unauthorized or no org member, check if org is visible
			accessMode = perm.AccessModeRead
		}
	} else {
		if doer != nil && !doer.IsGhost() {
			// 1. Check if user is package owner
			if doer.ID == pkgOwner.ID {
				accessMode = perm.AccessModeOwner
			} else if pkgOwner.Visibility.IsPublic() || (pkgOwner.Visibility.IsLimited() && !doer.IsRestricted) { // 2. Check if package owner is visible to the doer
				accessMode = perm.AccessModeRead
			}
		} else if pkgOwner.Visibility.IsPublic() { // 3. Check if package owner is public
			accessMode = perm.AccessModeRead
		}
	}

	return accessMode, nil
}

// PackageContexter initializes a package context for a request.
func PackageContexter() func(next http.Handler) http.Handler {
	renderer := templates.PageRenderer()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			base := NewBaseContext(resp, req)
			// FIXME: web Context is still needed when rendering 500 page in a package handler
			// It should be refactored to use new error handling mechanisms
			ctx := NewWebContext(base, renderer, nil)
			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}
