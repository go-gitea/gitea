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

	if ctx.Doer != nil && ctx.Doer.ID == ctx.ContextUser.ID {
		ctx.Package.AccessMode = perm.AccessModeOwner
	} else {
		if ctx.Package.Owner.IsOrganization() {
			if organization.HasOrgOrUserVisible(ctx, ctx.Package.Owner, ctx.Doer) {
				ctx.Package.AccessMode = perm.AccessModeRead
				if ctx.Doer != nil {
					var err error
					ctx.Package.AccessMode, err = organization.OrgFromUser(ctx.Package.Owner).GetOrgUserMaxAuthorizeLevel(ctx.Doer.ID)
					if err != nil {
						errCb(http.StatusInternalServerError, "GetOrgUserMaxAuthorizeLevel", err)
						return
					}
				}
			}
		} else {
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
