// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"net/http"
	"strconv"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/packages"
)

// Package contains owner, access mode and optional the package
type Package struct {
	Owner      *models.User
	AccessMode models.AccessMode
	Descriptor *packages.PackageDescriptor
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

	if ctx.IsSigned && ctx.User.ID == ctx.ContextUser.ID {
		ctx.Package.AccessMode = models.AccessModeOwner
	} else {
		if ctx.Package.Owner.IsOrganization() {
			if ctx.User != nil && models.HasOrgOrUserVisible(ctx.Package.Owner, ctx.User) {
				var err error
				ctx.Package.AccessMode, err = models.OrgFromUser(ctx.Package.Owner).GetOrgUserMaxAuthorizeLevel(ctx.User.ID)
				if err != nil {
					errCb(http.StatusInternalServerError, "GetOrgUserMaxAuthorizeLevel", err)
					return
				}
			}
		} else {
			ctx.Package.AccessMode = models.AccessModeRead
		}
	}

	versionID := ctx.Params(":versionid")
	if versionID != "" {
		id, err := strconv.ParseInt(versionID, 10, 64)
		if err != nil {
			errCb(http.StatusInternalServerError, "ParseInt", err)
			return
		}

		pv, err := packages.GetVersionByID(ctx, id)
		if err != nil {
			if err == packages.ErrPackageNotExist {
				errCb(http.StatusNotFound, "GetVersionByID", err)
			} else {
				errCb(http.StatusInternalServerError, "GetVersionByID", err)
			}
			return
		}

		ctx.Package.Descriptor, err = packages.GetPackageDescriptorCtx(ctx, pv)
		if err != nil {
			errCb(http.StatusInternalServerError, "GetPackageDescriptorCtx", err)
			return
		}

		if ctx.Package.Descriptor.Owner.ID != ctx.Package.Owner.ID {
			errCb(http.StatusNotFound, "Package owner does not match", "Package owner does not match")
			return
		}
	}
}
