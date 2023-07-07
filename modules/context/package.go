// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
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
		errorFn := func(status int, title string, obj any) {
			err, ok := obj.(error)
			if !ok {
				err = fmt.Errorf("%s", obj)
			}
			if status == http.StatusNotFound {
				ctx.NotFound(title, err)
			} else {
				ctx.ServerError(title, err)
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
		ctx.Package = packageAssignment(paCtx, ctx.Error)
	}
}

func packageAssignment(ctx *packageAssignmentCtx, errCb func(int, string, any)) *Package {
	pkg := &Package{
		Owner: ctx.ContextUser,
	}
	var err error
	pkg.AccessMode, err = determineAccessMode(ctx.Base, pkg, ctx.Doer)
	if err != nil {
		errCb(http.StatusInternalServerError, "determineAccessMode", err)
		return pkg
	}

	packageType := ctx.Params("type")
	name := ctx.Params("name")
	version := ctx.Params("version")
	if packageType != "" && name != "" && version != "" {
		pv, err := packages_model.GetVersionByNameAndVersion(ctx, pkg.Owner.ID, packages_model.Type(packageType), name, version)
		if err != nil {
			if err == packages_model.ErrPackageNotExist {
				errCb(http.StatusNotFound, "GetVersionByNameAndVersion", err)
			} else {
				errCb(http.StatusInternalServerError, "GetVersionByNameAndVersion", err)
			}
			return pkg
		}

		pkg.Descriptor, err = packages_model.GetPackageDescriptor(ctx, pv)
		if err != nil {
			errCb(http.StatusInternalServerError, "GetPackageDescriptor", err)
			return pkg
		}
	}

	return pkg
}

func determineAccessMode(ctx *Base, pkg *Package, doer *user_model.User) (perm.AccessMode, error) {
	if setting.Service.RequireSignInView && doer == nil {
		return perm.AccessModeNone, nil
	}

	if doer != nil && !doer.IsGhost() && (!doer.IsActive || doer.ProhibitLogin) {
		return perm.AccessModeNone, nil
	}

	// TODO: ActionUser permission check
	accessMode := perm.AccessModeNone
	if pkg.Owner.IsOrganization() {
		org := organization.OrgFromUser(pkg.Owner)

		if doer != nil && !doer.IsGhost() {
			// 1. If user is logged in, check all team packages permissions
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
		} else if organization.HasOrgOrUserVisible(ctx, pkg.Owner, doer) {
			// 2. If user is non-login, check if org is visible to non-login user
			accessMode = perm.AccessModeRead
		}
	} else {
		if doer != nil && !doer.IsGhost() {
			// 1. Check if user is package owner
			if doer.ID == pkg.Owner.ID {
				accessMode = perm.AccessModeOwner
			} else if pkg.Owner.Visibility == structs.VisibleTypePublic || pkg.Owner.Visibility == structs.VisibleTypeLimited { // 2. Check if package owner is public or limited
				accessMode = perm.AccessModeRead
			}
		} else if pkg.Owner.Visibility == structs.VisibleTypePublic { // 3. Check if package owner is public
			accessMode = perm.AccessModeRead
		}
	}

	return accessMode, nil
}

// PackageContexter initializes a package context for a request.
func PackageContexter() func(next http.Handler) http.Handler {
	renderer := templates.HTMLRenderer()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			base, baseCleanUp := NewBaseContext(resp, req)
			ctx := &Context{
				Base:   base,
				Render: renderer, // it is still needed when rendering 500 page in a package handler
			}
			defer baseCleanUp()

			ctx.Base.AppendContextValue(WebContextKey, ctx)
			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}
