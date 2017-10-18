// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	"fmt"

	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/user"
)

// listMembers list an organization's members
func listMembers(ctx *context.APIContext, publicOnly bool) {
	var members []*models.User
	if publicOnly {
		orgUsers, err := models.GetOrgUsersByOrgID(ctx.Org.Organization.ID)
		if err != nil {
			ctx.Error(500, "GetOrgUsersByOrgID", err)
			return
		}

		memberIDs := make([]int64, 0, len(orgUsers))
		for _, orgUser := range orgUsers {
			if orgUser.IsPublic {
				memberIDs = append(memberIDs, orgUser.UID)
			}
		}

		if members, err = models.GetUsersByIDs(memberIDs); err != nil {
			ctx.Error(500, "GetUsersByIDs", err)
			return
		}
	} else {
		if err := ctx.Org.Organization.GetMembers(); err != nil {
			ctx.Error(500, "GetMembers", err)
			return
		}
		members = ctx.Org.Organization.Members
	}

	apiMembers := make([]*api.User, len(members))
	for i, member := range members {
		apiMembers[i] = member.APIFormat()
	}
	ctx.JSON(200, apiMembers)
}

// ListMembers list an organization's members
func ListMembers(ctx *context.APIContext) {
	// swagger:route GET /orgs/{orgname}/members organization orgListMembers
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: UserList
	//       500: error

	publicOnly := ctx.User == nil || !ctx.Org.Organization.IsOrgMember(ctx.User.ID)
	listMembers(ctx, publicOnly)
}

// ListPublicMembers list an organization's public members
func ListPublicMembers(ctx *context.APIContext) {
	// swagger:route GET /orgs/{orgname}/public_members organization orgListPublicMembers
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: UserList
	//       500: error

	listMembers(ctx, true)
}

// IsMember check if a user is a member of an organization
func IsMember(ctx *context.APIContext) {
	// swagger:route GET /orgs/{orgname}/members/{username} organization orgIsMember
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       204: empty
	//       302: redirect
	//       404: notFound

	userToCheck := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if ctx.User != nil && ctx.Org.Organization.IsOrgMember(ctx.User.ID) {
		if ctx.Org.Organization.IsOrgMember(userToCheck.ID) {
			ctx.Status(204)
		} else {
			ctx.Status(404)
		}
	} else if ctx.User != nil && ctx.User.ID == userToCheck.ID {
		ctx.Status(404)
	} else {
		redirectURL := fmt.Sprintf("%sapi/v1/orgs/%s/public_members/%s",
			setting.AppURL, ctx.Org.Organization.Name, userToCheck.Name)
		ctx.Redirect(redirectURL, 302)
	}
}

// IsPublicMember check if a user is a public member of an organization
func IsPublicMember(ctx *context.APIContext) {
	// swagger:route GET /orgs/{orgname}/public_members/{username} organization orgIsPublicMember
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       204: empty
	//       404: notFound

	userToCheck := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if userToCheck.IsPublicMember(ctx.Org.Organization.ID) {
		ctx.Status(204)
	} else {
		ctx.Status(404)
	}
}

// PublicizeMember make a member's membership public
func PublicizeMember(ctx *context.APIContext) {
	// swagger:route PUT /orgs/{orgname}/public_members/{username} organization orgPublicizeMember
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       204: empty
	//       403: forbidden
	//       500: error

	userToPublicize := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if userToPublicize.ID != ctx.User.ID {
		ctx.Error(403, "", "Cannot publicize another member")
		return
	}
	err := models.ChangeOrgUserStatus(ctx.Org.Organization.ID, userToPublicize.ID, true)
	if err != nil {
		ctx.Error(500, "ChangeOrgUserStatus", err)
		return
	}
	ctx.Status(204)
}

// ConcealMember make a member's membership not public
func ConcealMember(ctx *context.APIContext) {
	// swagger:route DELETE /orgs/{orgname}/public_members/{username} organization orgConcealMember
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       204: empty
	//       403: forbidden
	//       500: error

	userToConceal := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if userToConceal.ID != ctx.User.ID {
		ctx.Error(403, "", "Cannot conceal another member")
		return
	}
	err := models.ChangeOrgUserStatus(ctx.Org.Organization.ID, userToConceal.ID, false)
	if err != nil {
		ctx.Error(500, "ChangeOrgUserStatus", err)
		return
	}
	ctx.Status(204)
}

// DeleteMember remove a member from an organization
func DeleteMember(ctx *context.APIContext) {
	// swagger:route DELETE /orgs/{orgname}/members/{username} organization orgDeleteMember
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       204: empty
	//       500: error

	member := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if err := ctx.Org.Organization.RemoveMember(member.ID); err != nil {
		ctx.Error(500, "RemoveMember", err)
	}
	ctx.Status(204)
}
