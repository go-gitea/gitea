// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/user"
)

// listMembers list an organization's members
func listMembers(ctx *context.APIContext, publicOnly bool) {
	var members []*models.User
	members, _, err := models.FindOrgMembers(models.FindOrgMembersOpts{
		OrgID:      ctx.Org.Organization.ID,
		PublicOnly: publicOnly,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUsersByIDs", err)
		return
	}

	apiMembers := make([]*api.User, len(members))
	for i, member := range members {
		apiMembers[i] = convert.ToUser(member, ctx.IsSigned, ctx.User != nil && ctx.User.IsAdmin)
	}
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"

	publicOnly := true
	if ctx.User != nil {
		isMember, err := ctx.Org.Organization.IsOrgMember(ctx.User.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "IsOrgMember", err)
			return
		}
		publicOnly = !isMember
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
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"

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
	//   "302":
	//     description: redirection to /orgs/{org}/public_members/{username}
	//   "404":
	//     description: user is not a member

	userToCheck := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if ctx.User != nil {
		userIsMember, err := ctx.Org.Organization.IsOrgMember(ctx.User.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "IsOrgMember", err)
			return
		} else if userIsMember {
			userToCheckIsMember, err := ctx.Org.Organization.IsOrgMember(userToCheck.ID)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "IsOrgMember", err)
			} else if userToCheckIsMember {
				ctx.Status(http.StatusNoContent)
			} else {
				ctx.NotFound()
			}
			return
		} else if ctx.User.ID == userToCheck.ID {
			ctx.NotFound()
			return
		}
	}

	redirectURL := fmt.Sprintf("%sapi/v1/orgs/%s/public_members/%s",
		setting.AppURL, ctx.Org.Organization.Name, userToCheck.Name)
	ctx.Redirect(redirectURL, 302)
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
	if userToCheck.IsPublicMember(ctx.Org.Organization.ID) {
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

	userToPublicize := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if userToPublicize.ID != ctx.User.ID {
		ctx.Error(http.StatusForbidden, "", "Cannot publicize another member")
		return
	}
	err := models.ChangeOrgUserStatus(ctx.Org.Organization.ID, userToPublicize.ID, true)
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

	userToConceal := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if userToConceal.ID != ctx.User.ID {
		ctx.Error(http.StatusForbidden, "", "Cannot conceal another member")
		return
	}
	err := models.ChangeOrgUserStatus(ctx.Org.Organization.ID, userToConceal.ID, false)
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

	member := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if err := ctx.Org.Organization.RemoveMember(member.ID); err != nil {
		ctx.Error(http.StatusInternalServerError, "RemoveMember", err)
	}
	ctx.Status(http.StatusNoContent)
}
