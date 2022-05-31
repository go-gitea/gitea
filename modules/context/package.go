// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/perm"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"
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

	if ctx.Package.Owner.IsOrganization() {
		// 1. Get user max authorize level for the org (may be none, if user is not member of the org)
		if ctx.Doer != nil {
			var err error
			ctx.Package.AccessMode, err = organization.OrgFromUser(ctx.Package.Owner).GetOrgUserMaxAuthorizeLevel(ctx.Doer.ID)
			if err != nil {
				errCb(http.StatusInternalServerError, "GetOrgUserMaxAuthorizeLevel", err)
				return
			}
		}
		// 2. If authorize level is none, check if org is visible to user
		if ctx.Package.AccessMode == perm.AccessModeNone && organization.HasOrgOrUserVisible(ctx, ctx.Package.Owner, ctx.Doer) {
			ctx.Package.AccessMode = perm.AccessModeRead
		}
	} else {
		if ctx.Doer != nil && !ctx.Doer.IsGhost() {
			// 1. Check if user is package owner
			if ctx.Doer.ID == ctx.Package.Owner.ID {
				ctx.Package.AccessMode = perm.AccessModeOwner
			} else if ctx.Package.Owner.Visibility == structs.VisibleTypePublic || ctx.Package.Owner.Visibility == structs.VisibleTypeLimited { // 2. Check if package owner is public or limited
				ctx.Package.AccessMode = perm.AccessModeRead
			}
		} else if ctx.Package.Owner.Visibility == structs.VisibleTypePublic { // 3. Check if package owner is public
			ctx.Package.AccessMode = perm.AccessModeRead
		}
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

// PackageContexter initializes a package context for a request.
func PackageContexter() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			ctx := Context{
				Resp: NewResponse(resp),
				Data: map[string]interface{}{},
			}
			defer ctx.Close()

			ctx.Req = WithContext(req, &ctx)

			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}
