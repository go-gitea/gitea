// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"net/http"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	org_service "gitea.dev/services/org"
)

// ListSubgroups lists direct or recursive descendant subgroups of an organization.
func ListSubgroups(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/subgroups organization orgListSubgroups
	// ---
	// summary: List an organization's subgroups
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the parent organization
	//   type: string
	//   required: true
	// - name: recursive
	//   in: query
	//   description: include all nested descendant subgroups recursively
	//   type: boolean
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/OrganizationList"
	//   default:
	//     "$ref": "#/responses/error"

	if !organization.HasOrgOrUserVisible(ctx, ctx.Org.Organization.AsUser(), ctx.Doer) {
		ctx.APIErrorNotFound()
		return
	}

	listOptions := utils.GetListOptions(ctx)
	var subgroups []*organization.Organization
	var total int64

	if ctx.FormBool("recursive") {
		allDesc, err := organization.GetDescendantOrgs(ctx, ctx.Org.Organization.ID, ctx.Doer)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		total = int64(len(allDesc))
		if listOptions.PageSize > 0 {
			page := max(listOptions.Page, 1)
			start := (page - 1) * listOptions.PageSize
			if start < len(allDesc) {
				end := min(start+listOptions.PageSize, len(allDesc))
				subgroups = allDesc[start:end]
			} else {
				subgroups = []*organization.Organization{}
			}
		} else {
			subgroups = allDesc
		}
	} else {
		var err error
		subgroups, total, err = organization.GetSubgroups(ctx, ctx.Org.Organization.ID, ctx.Doer, listOptions)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	apiOrgs := make([]*api.Organization, len(subgroups))
	for i := range subgroups {
		apiOrgs[i] = convert.ToOrganization(ctx, subgroups[i])
		if ctx.Doer == nil {
			apiOrgs[i].Email = ""
		}
	}

	ctx.SetLinkHeader(total, listOptions.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, &apiOrgs)
}

// CreateSubgroup creates a child subgroup under an existing parent organization.
func CreateSubgroup(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/subgroups organization orgCreateSubgroup
	// ---
	// summary: Create a subgroup under an organization
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the parent organization
	//   type: string
	//   required: true
	// - name: subgroup
	//   in: body
	//   required: true
	//   schema: { "$ref": "#/definitions/CreateSubgroupOption" }
	// responses:
	//   "201":
	//     "$ref": "#/responses/Organization"
	//   default:
	//     "$ref": "#/responses/error"

	form := web.GetForm[*api.CreateSubgroupOption](ctx)
	parentOrg := ctx.Org.Organization

	visibility := parentOrg.Visibility
	if form.Visibility != "" {
		visibility = api.VisibilityModes[form.Visibility]
	}

	subOrg := &organization.Organization{
		Name:                      form.UserName,
		FullName:                  form.FullName,
		Email:                     form.Email,
		Description:               form.Description,
		Website:                   form.Website,
		Location:                  form.Location,
		IsActive:                  true,
		Type:                      user_model.UserTypeOrganization,
		Visibility:                visibility,
		RepoAdminChangeTeamAccess: form.RepoAdminChangeTeamAccess,
	}

	if err := org_service.CreateSubgroup(ctx, parentOrg, subOrg, ctx.Doer); err != nil {
		switch {
		case organization.IsErrUserNotAllowedCreateOrg(err), errors.Is(err, util.ErrPermissionDenied):
			ctx.APIError(http.StatusForbidden, err.Error())
		case user_model.IsErrUserAlreadyExist(err),
			db.IsErrNameReserved(err),
			db.IsErrNameCharsNotAllowed(err),
			db.IsErrNamePatternNotAllowed(err),
			errors.Is(err, util.ErrInvalidArgument):
			ctx.APIError(http.StatusUnprocessableEntity, err.Error())
		default:
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToOrganization(ctx, subOrg))
}

// ListAncestors returns the ordered chain of ancestor organizations from root to immediate parent.
func ListAncestors(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/ancestors organization orgListAncestors
	// ---
	// summary: List an organization's ancestor organizations
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/OrganizationList"
	//   default:
	//     "$ref": "#/responses/error"

	if !organization.HasOrgOrUserVisible(ctx, ctx.Org.Organization.AsUser(), ctx.Doer) {
		ctx.APIErrorNotFound()
		return
	}

	ancestors, err := organization.GetAncestorOrgs(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	visibleAncestors := make([]*api.Organization, 0, len(ancestors))
	for _, anc := range ancestors {
		if organization.HasOrgOrUserVisible(ctx, anc.AsUser(), ctx.Doer) {
			apiAnc := convert.ToOrganization(ctx, anc)
			if ctx.Doer == nil {
				apiAnc.Email = ""
			}
			visibleAncestors = append(visibleAncestors, apiAnc)
		}
	}

	ctx.JSON(http.StatusOK, &visibleAncestors)
}

// MoveSubgroup moves an organization under a new parent organization or detaches it to top-level.
func MoveSubgroup(ctx *context.APIContext) {
	// swagger:operation PATCH /orgs/{org}/parent organization orgMoveSubgroup
	// ---
	// summary: Move or detach an organization subgroup
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the subgroup organization to move
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema: { "$ref": "#/definitions/MoveSubgroupOption" }
	// responses:
	//   "200":
	//     "$ref": "#/responses/Organization"
	//   default:
	//     "$ref": "#/responses/error"

	form := web.GetForm[*api.MoveSubgroupOption](ctx)
	var newParent *organization.Organization
	if form.ParentOrg != "" {
		var err error
		newParent, err = organization.GetOrgByName(ctx, form.ParentOrg)
		if err != nil {
			if organization.IsErrOrgNotExist(err) {
				ctx.APIErrorNotFound(err.Error())
			} else {
				ctx.APIErrorInternal(err)
			}
			return
		}
	}

	if err := org_service.MoveSubgroup(ctx, ctx.Org.Organization, newParent, ctx.Doer); err != nil {
		switch {
		case organization.IsErrUserNotAllowedCreateOrg(err), errors.Is(err, util.ErrPermissionDenied):
			ctx.APIError(http.StatusForbidden, err.Error())
		case errors.Is(err, util.ErrInvalidArgument):
			ctx.APIError(http.StatusUnprocessableEntity, err.Error())
		default:
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.JSON(http.StatusOK, convert.ToOrganization(ctx, ctx.Org.Organization))
}
