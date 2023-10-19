// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"
	"net/url"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/user"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
)

// listMembers list an organization's members
func listMembers(ctx *context.APIContext, publicOnly bool) {
	opts := &organization.FindOrgMembersOpts{
		OrgID:       ctx.Org.Organization.ID,
		PublicOnly:  publicOnly,
		ListOptions: utils.GetListOptions(ctx),
	}

	count, err := organization.CountOrgMembers(ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	members, _, err := organization.FindOrgMembers(ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	apiMembers := make([]*api.User, len(members))
	for i, member := range members {
		apiMembers[i] = convert.ToUser(ctx, member, ctx.Doer)
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiMembers)
}

// ListMembers list an organization's members
func ListMembers(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/members organization orgListMembers
	// ---
	// summary: List an organization's members
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
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
	//     "$ref": "#/responses/UserList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	publicOnly := true
	if ctx.Doer != nil {
		isMember, err := ctx.Org.Organization.IsOrgMember(ctx, ctx.Doer.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "IsOrgMember", err)
			return
		}
		publicOnly = !isMember && !ctx.Doer.IsAdmin
	}
	listMembers(ctx, publicOnly)
}

// ListPublicMembers list an organization's public members
func ListPublicMembers(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/public_members organization orgListPublicMembers
	// ---
	// summary: List an organization's public members
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	listMembers(ctx, true)
}

// IsMember check if a user is a member of an organization
func IsMember(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/members/{username} organization orgIsMember
	// ---
	// summary: Check if a user is a member of an organization
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: user is a member
	//   "303":
	//     description: redirection to /orgs/{org}/public_members/{username}
	//   "404":
	//     description: user is not a member

	userToCheck := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if ctx.Doer != nil {
		userIsMember, err := ctx.Org.Organization.IsOrgMember(ctx, ctx.Doer.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "IsOrgMember", err)
			return
		} else if userIsMember || ctx.Doer.IsAdmin {
			userToCheckIsMember, err := ctx.Org.Organization.IsOrgMember(ctx, userToCheck.ID)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "IsOrgMember", err)
			} else if userToCheckIsMember {
				ctx.Status(http.StatusNoContent)
			} else {
				ctx.NotFound()
			}
			return
		} else if ctx.Doer.ID == userToCheck.ID {
			ctx.NotFound()
			return
		}
	}

	redirectURL := setting.AppSubURL + "/api/v1/orgs/" + url.PathEscape(ctx.Org.Organization.Name) + "/public_members/" + url.PathEscape(userToCheck.Name)
	ctx.Redirect(redirectURL)
}

// IsPublicMember check if a user is a public member of an organization
func IsPublicMember(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/public_members/{username} organization orgIsPublicMember
	// ---
	// summary: Check if a user is a public member of an organization
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: user is a public member
	//   "404":
	//     description: user is not a public member

	userToCheck := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	is, err := organization.IsPublicMembership(ctx, ctx.Org.Organization.ID, userToCheck.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "IsPublicMembership", err)
		return
	}
	if is {
		ctx.Status(http.StatusNoContent)
	} else {
		ctx.NotFound()
	}
}

// PublicizeMember make a member's membership public
func PublicizeMember(ctx *context.APIContext) {
	// swagger:operation PUT /orgs/{org}/public_members/{username} organization orgPublicizeMember
	// ---
	// summary: Publicize a user's membership
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: membership publicized
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	userToPublicize := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if userToPublicize.ID != ctx.Doer.ID {
		ctx.Error(http.StatusForbidden, "", "Cannot publicize another member")
		return
	}
	err := organization.ChangeOrgUserStatus(ctx, ctx.Org.Organization.ID, userToPublicize.ID, true)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ChangeOrgUserStatus", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ConcealMember make a member's membership not public
func ConcealMember(ctx *context.APIContext) {
	// swagger:operation DELETE /orgs/{org}/public_members/{username} organization orgConcealMember
	// ---
	// summary: Conceal a user's membership
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	userToConceal := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if userToConceal.ID != ctx.Doer.ID {
		ctx.Error(http.StatusForbidden, "", "Cannot conceal another member")
		return
	}
	err := organization.ChangeOrgUserStatus(ctx, ctx.Org.Organization.ID, userToConceal.ID, false)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ChangeOrgUserStatus", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// DeleteMember remove a member from an organization
func DeleteMember(ctx *context.APIContext) {
	// swagger:operation DELETE /orgs/{org}/members/{username} organization orgDeleteMember
	// ---
	// summary: Remove a member from an organization
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: member removed
	//   "404":
	//     "$ref": "#/responses/notFound"

	member := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if err := models.RemoveOrgUser(ctx, ctx.Org.Organization.ID, member.ID); err != nil {
		ctx.Error(http.StatusInternalServerError, "RemoveOrgUser", err)
	}
	ctx.Status(http.StatusNoContent)
}
