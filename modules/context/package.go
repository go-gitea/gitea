// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	gocontext "context"
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

// PackageAssignment returns a middleware to handle Context.Package assignment
func PackageAssignment() func(ctx *Context) {
	return func(ctx *Context) {
		packageAssignment(ctx, func(status int, title string, obj interface{}) {
			err, ok := obj.(error)
			if !ok {
				err = fmt.Errorf("%s", obj)
			}
			if status == http.StatusNotFound {
				ctx.NotFound(title, err)
			} else {
				ctx.ServerError(title, err)
			}
		})
	}
}

// PackageAssignmentAPI returns a middleware to handle Context.Package assignment
func PackageAssignmentAPI() func(ctx *APIContext) {
	return func(ctx *APIContext) {
		packageAssignment(ctx.Context, ctx.Error)
	}
}

func packageAssignment(ctx *Context, errCb func(int, string, interface{})) {
	ctx.Package = &Package{
		Owner: ctx.ContextUser,
	}

	var err error
	ctx.Package.AccessMode, err = determineAccessMode(ctx)
	if err != nil {
		errCb(http.StatusInternalServerError, "determineAccessMode", err)
		return
	}

	packageType := ctx.Params("type")
	name := ctx.Params("name")
	version := ctx.Params("version")
	if packageType != "" && name != "" && version != "" {
		pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.Type(packageType), name, version)
		if err != nil {
			if err == packages_model.ErrPackageNotExist {
				errCb(http.StatusNotFound, "GetVersionByNameAndVersion", err)
			} else {
				errCb(http.StatusInternalServerError, "GetVersionByNameAndVersion", err)
			}
			return
		}

		ctx.Package.Descriptor, err = packages_model.GetPackageDescriptor(ctx, pv)
		if err != nil {
			errCb(http.StatusInternalServerError, "GetPackageDescriptor", err)
			return
		}
	}
}

func determineAccessMode(ctx *Context) (perm.AccessMode, error) {
	if setting.Service.RequireSignInView && ctx.Doer == nil {
		return perm.AccessModeNone, nil
	}

	if ctx.Doer != nil && !ctx.Doer.IsGhost() && (!ctx.Doer.IsActive || ctx.Doer.ProhibitLogin) {
		return perm.AccessModeNone, nil
	}

	accessMode := perm.AccessModeNone
	if ctx.Package.Owner.IsOrganization() {
		org := organization.OrgFromUser(ctx.Package.Owner)

		// 1. Get user max authorize level for the org (may be none, if user is not member of the org)
		if ctx.Doer != nil {
			var err error
			accessMode, err = org.GetOrgUserMaxAuthorizeLevel(ctx.Doer.ID)
			if err != nil {
				return accessMode, err
			}
			// If access mode is less than write check every team for more permissions
			if accessMode < perm.AccessModeWrite {
				teams, err := organization.GetUserOrgTeams(ctx, org.ID, ctx.Doer.ID)
				if err != nil {
					return accessMode, err
				}
				for _, t := range teams {
					perm := t.UnitAccessModeCtx(ctx, unit.TypePackages)
					if accessMode < perm {
						accessMode = perm
					}
				}
			}
		}
		// 2. If authorize level is none, check if org is visible to user
		if accessMode == perm.AccessModeNone && organization.HasOrgOrUserVisible(ctx, ctx.Package.Owner, ctx.Doer) {
			accessMode = perm.AccessModeRead
		}
	} else {
		if ctx.Doer != nil && !ctx.Doer.IsGhost() {
			// 1. Check if user is package owner
			if ctx.Doer.ID == ctx.Package.Owner.ID {
				accessMode = perm.AccessModeOwner
			} else if ctx.Package.Owner.Visibility == structs.VisibleTypePublic || ctx.Package.Owner.Visibility == structs.VisibleTypeLimited { // 2. Check if package owner is public or limited
				accessMode = perm.AccessModeRead
			}
		} else if ctx.Package.Owner.Visibility == structs.VisibleTypePublic { // 3. Check if package owner is public
			accessMode = perm.AccessModeRead
		}
	}

	return accessMode, nil
}

// PackageContexter initializes a package context for a request.
func PackageContexter(ctx gocontext.Context) func(next http.Handler) http.Handler {
	_, rnd := templates.HTMLRenderer(ctx)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			ctx := Context{
				Resp:   NewResponse(resp),
				Data:   map[string]interface{}{},
				Render: rnd,
			}
			defer ctx.Close()

			ctx.Req = WithContext(req, &ctx)

			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}
