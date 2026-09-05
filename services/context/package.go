// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"errors"
	"fmt"
	"net/http"

	"gitea.dev/models/organization"
	packages_model "gitea.dev/models/packages"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
)

// Package contains owner, access mode and optional the package descriptor
type Package struct {
	Owner      *user_model.User
	AccessMode perm.AccessMode
	Descriptor *packages_model.PackageDescriptor
}

type packageAssignmentCtx struct {
	*Base
	Doer        *user_model.User
	ContextUser *user_model.User
	Package     *packages_model.Package
	Repository  *repo_model.Repository
}

// PackageAssignment returns a middleware to handle Context.Package assignment
func PackageAssignment(pType string) func(ctx *Context) {
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
		var pkg *packages_model.Package
		var err error
		switch pType {
		case "web":
			pkg, err = packages_model.GetPackageByName(paCtx, ctx.ContextUser.ID, packages_model.Type(ctx.PathParam("type")), ctx.PathParam("name"))
		case "container":
			pkg, err = packages_model.GetPackageByName(paCtx, ctx.ContextUser.ID, packages_model.TypeContainer, ctx.PathParam("image"))
		case "terraform":
			pkg, err = packages_model.GetPackageByName(paCtx, ctx.ContextUser.ID, packages_model.TypeTerraformState, ctx.PathParam("name"))
		}
		if err == nil {
			paCtx.Package = pkg
			if pkg.RepoID != 0 {
				repo, err := repo_model.GetRepositoryByID(ctx, pkg.RepoID)
				if err == nil {
					paCtx.Repository = repo
				}
			}
		}
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
	accessMode, err := determineAccessMode(ctx)
	if err != nil {
		errCb(http.StatusInternalServerError, fmt.Sprintf("determineAccessMode: %v", err))
		return nil
	}

	pkg := &Package{
		Owner:      ctx.ContextUser,
		AccessMode: accessMode,
	}
	packageType := ctx.PathParam("type")
	name := ctx.PathParam("name")
	if packageType == "" || name == "" {
		return pkg
	}

	version := ctx.PathParam("version")
	if version != "" {
		pv, err := packages_model.GetVersionByNameAndVersion(ctx, pkg.Owner.ID, packages_model.Type(packageType), name, version)
		if err != nil {
			if errors.Is(err, packages_model.ErrPackageNotExist) {
				errCb(http.StatusNotFound, fmt.Sprintf("GetVersionByNameAndVersion: %v", err))
			} else {
				errCb(http.StatusInternalServerError, fmt.Sprintf("GetVersionByNameAndVersion: %v", err))
			}
			return pkg
		}

		pkg.Descriptor, err = packages_model.GetPackageDescriptor(ctx, pv)
		if err != nil {
			errCb(http.StatusInternalServerError, fmt.Sprintf("GetPackageDescriptor: %v", err))
			return pkg
		}
	} else {
		p, err := packages_model.GetPackageByName(ctx, pkg.Owner.ID, packages_model.Type(packageType), name)
		if err != nil {
			if errors.Is(err, packages_model.ErrPackageNotExist) {
				errCb(http.StatusNotFound, fmt.Sprintf("GetPackageByName: %v", err))
			} else {
				errCb(http.StatusInternalServerError, fmt.Sprintf("GetPackageByName: %v", err))
			}
			return pkg
		}

		pkg.Descriptor = &packages_model.PackageDescriptor{
			Package: p,
			Owner:   pkg.Owner,
		}
	}

	return pkg
}

func determineAccessMode(ctx *packageAssignmentCtx) (perm.AccessMode, error) {
	doer := ctx.Doer
	pkgOwner := ctx.ContextUser
	repo := ctx.Repository
	if setting.Service.RequireSignInViewStrict && (doer == nil || doer.IsGhost()) {
		return perm.AccessModeNone, nil
	}

	if doer != nil && !doer.IsGhost() && (!doer.IsActive || doer.ProhibitLogin) {
		return perm.AccessModeNone, nil
	}

	// TODO: ActionUser permission check
	accessMode := perm.AccessModeNone
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
			if repo != nil {
				// 3. If package is associated with a repository, check if repository is visible
				if !repo.IsPrivate {
					accessMode = perm.AccessModeRead
				}
			} else {
				accessMode = perm.AccessModeRead
			}
		}
	} else {
		if doer != nil && !doer.IsGhost() {
			// 1. Check if user is package owner
			if doer.ID == pkgOwner.ID {
				accessMode = perm.AccessModeOwner
			} else if pkgOwner.Visibility.IsPublic() || (pkgOwner.Visibility.IsLimited() && !doer.IsRestricted) { // 2. Check if package owner is visible to the doer
				if repo != nil {
					// 3. If package is associated with a repository, check if repository is visible
					if !repo.IsPrivate {
						accessMode = perm.AccessModeRead
					}
				} else {
					accessMode = perm.AccessModeRead
				}
			}
		} else if pkgOwner.Visibility.IsPublic() { // 3. Check if package owner is public
			if repo != nil {
				// 3. If package is associated with a repository, check if repository is visible
				if !repo.IsPrivate {
					accessMode = perm.AccessModeRead
				}
			} else {
				accessMode = perm.AccessModeRead
			}
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
