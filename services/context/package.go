// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"errors"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/templates"
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
}

// PackageAssignment returns a middleware to handle Context.Package assignment
func PackageAssignment() func(ctx *Context) {
	return func(ctx *Context) {
		errorFn := func(status int, obj any) {
			err, ok := obj.(error)
			if !ok {
				err = fmt.Errorf("%s", obj)
			}
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

func packageAssignment(ctx *packageAssignmentCtx, errCb func(int, any)) *Package {
	pkgOwner := ctx.ContextUser
	accessMode, err := determineAccessMode(ctx.Base, pkgOwner, ctx.Doer)
	if err != nil {
		errCb(http.StatusInternalServerError, fmt.Errorf("determineAccessMode: %w", err))
		return nil
	}

	pkg := &Package{
		Owner:      pkgOwner,
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
				errCb(http.StatusNotFound, fmt.Errorf("GetVersionByNameAndVersion: %w", err))
			} else {
				errCb(http.StatusInternalServerError, fmt.Errorf("GetVersionByNameAndVersion: %w", err))
			}
			return pkg
		}

		pkg.Descriptor, err = packages_model.GetPackageDescriptor(ctx, pv)
		if err != nil {
			errCb(http.StatusInternalServerError, fmt.Errorf("GetPackageDescriptor: %w", err))
			return pkg
		}
	} else {
		p, err := packages_model.GetPackageByName(ctx, pkg.Owner.ID, packages_model.Type(packageType), name)
		if err != nil {
			if errors.Is(err, packages_model.ErrPackageNotExist) {
				errCb(http.StatusNotFound, fmt.Errorf("GetPackageByName: %w", err))
			} else {
				errCb(http.StatusInternalServerError, fmt.Errorf("GetPackageByName: %w", err))
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

func determineAccessMode(ctx *Base, pkgOwner, doer *user_model.User) (perm.AccessMode, error) {
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
			accessMode = perm.AccessModeRead
		}
	} else {
		if doer != nil && !doer.IsGhost() {
			// 1. Check if user is package owner
			if doer.ID == pkgOwner.ID {
				accessMode = perm.AccessModeOwner
			} else if pkgOwner.Visibility == structs.VisibleTypePublic || pkgOwner.Visibility == structs.VisibleTypeLimited { // 2. Check if package owner is public or limited
				accessMode = perm.AccessModeRead
			}
		} else if pkgOwner.Visibility == structs.VisibleTypePublic { // 3. Check if package owner is public
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
