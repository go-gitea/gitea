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
func ListMyOrgs(ctx *context.APIContext) {
	// swagger:operation GET /user/orgs organization orgListCurrentUserOrgs
	// ---
	// summary: List the current user's organizations
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/OrganizationList"
	listUserOrgs(ctx, ctx.User, true)
}

// ListUserOrgs list user's orgs
func ListUserOrgs(ctx *context.APIContext) {
	// swagger:operation GET /user/{username}/orgs organization orgListUserOrgs
	// ---
	// summary: List a user's organizations
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/OrganizationList"
	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	listUserOrgs(ctx, u, false)
}

// Get get an organization
func Get(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org} organization orgGet
	// ---
	// summary: Get an organization
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization to get
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Organization"
	ctx.JSON(200, convert.ToOrganization(ctx.Org.Organization))
}

// Edit change an organization's information
func Edit(ctx *context.APIContext, form api.EditOrgOption) {
	// swagger:operation PATCH /orgs/{org} organization orgEdit
	// ---
	// summary: Edit an organization
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization to edit
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditOrgOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Organization"
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
