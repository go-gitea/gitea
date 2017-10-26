// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/api/v1/convert"
	"code.gitea.io/gitea/routers/api/v1/user"
)

func listUserOrgs(ctx *context.APIContext, u *models.User, all bool) {
	if err := u.GetOrganizations(all); err != nil {
		ctx.Error(500, "GetOrganizations", err)
		return
	}

	apiOrgs := make([]*api.Organization, len(u.Orgs))
	for i := range u.Orgs {
		apiOrgs[i] = convert.ToOrganization(u.Orgs[i])
	}
	ctx.JSON(200, &apiOrgs)
}

// ListMyOrgs list all my orgs
// see https://github.com/gogits/go-gogs-client/wiki/Organizations#list-your-organizations
func ListMyOrgs(ctx *context.APIContext) {
	listUserOrgs(ctx, ctx.User, true)
}

// ListUserOrgs list user's orgs
// see https://github.com/gogits/go-gogs-client/wiki/Organizations#list-user-organizations
func ListUserOrgs(ctx *context.APIContext) {
	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	listUserOrgs(ctx, u, false)
}

// Get get an organization
// see https://github.com/gogits/go-gogs-client/wiki/Organizations#get-an-organization
func Get(ctx *context.APIContext) {
	ctx.JSON(200, convert.ToOrganization(ctx.Org.Organization))
}

// Edit change an organization's information
// see https://github.com/gogits/go-gogs-client/wiki/Organizations#edit-an-organization
func Edit(ctx *context.APIContext, form api.EditOrgOption) {
	org := ctx.Org.Organization
	org.FullName = form.FullName
	org.Description = form.Description
	org.Website = form.Website
	org.Location = form.Location
	if err := models.UpdateUserCols(org, "full_name", "description", "website", "location"); err != nil {
		ctx.Error(500, "UpdateUser", err)
		return
	}

	ctx.JSON(200, convert.ToOrganization(org))
}
